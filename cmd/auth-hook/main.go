package main

import (
	"context"
	"log"
	"net"
	"os"
	"strings"
	"text/template"
	"time"

	_ "embed"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
)

var (
	//go:embed wrapper.sh
	templateContent string

	templateWrapper = template.Must(template.New("").Parse(templateContent))
)

func main() {
	pflag.String("record-type", "TXT", "")
	pflag.String("record-line", "默认", "")
	pflag.Duration("timeout", time.Second*600, "resolution timeout")
	pflag.Duration("extra-wait", time.Second*100, "sometimes dns relays contains old value. wait for a period of time")

	pflag.String("secret-id", "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", "")
	pflag.String("secret-key", "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY", "")
	pflag.String("root-domain", "example.com", "")

	pflag.Bool("debug", false, "")
	pflag.Bool("wrap-self", false, "print script to wrap it self")
	pflag.Parse()

	pflag.VisitAll(func(f *pflag.Flag) {
		viper.BindEnv(strings.ReplaceAll(f.Name, "-", "_"))
	})

	viper.BindPFlags(pflag.CommandLine)

	viper.SetConfigName("authhook")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc")
	if path, err := os.UserHomeDir(); err == nil {
		viper.AddConfigPath(path)
	}

	viper.SetEnvPrefix("AUTHHOOK")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(err)
		}
	}

	recordResolutionTimeout := viper.GetDuration("timeout")
	extraWait := viper.GetDuration("extra-wait")
	recordType := viper.GetString("record-type")
	recordLine := viper.GetString("record-line")

	secretId := viper.GetString("secret-id")
	secretKey := viper.GetString("secret-key")
	rootDomain := viper.GetString("root-domain")

	if debug := viper.GetBool("debug"); debug {
		viper.DebugTo(os.Stderr)
	}

	if wrapSelf := viper.GetBool("wrap-self"); wrapSelf {
		templateWrapper.Execute(os.Stdout, map[string]any{
			"self": os.Args[0],

			"timeout":    recordResolutionTimeout,
			"extraWait":  extraWait,
			"recordType": recordType,
			"recordLine": recordLine,

			"secretId":   secretId,
			"secretKey":  secretKey,
			"rootDomain": rootDomain,
		})
		return
	}

	certbotDomain := os.Getenv("CERTBOT_DOMAIN")
	acmeChallegeValue := os.Getenv("CERTBOT_VALIDATION")
	acmeChangeDomain := "_acme-challenge." + certbotDomain
	acmeRecordName := strings.ReplaceAll(acmeChangeDomain, "."+rootDomain, "")

	log.Println("root domain: ", rootDomain)
	log.Println("certbot domain: ", certbotDomain)
	log.Println("challege domain: ", acmeChangeDomain)
	log.Println("record name: ", acmeRecordName)
	log.Println("record value: ", acmeChallegeValue)

	credential := common.NewCredential(secretId, secretKey)
	clientProfile := profile.NewClientProfile()
	client, err := dnspod.NewClient(credential, regions.Guangzhou, clientProfile)
	if err != nil {
		panic(err)
	}

	recordListRequest := dnspod.NewDescribeRecordListRequest()
	recordListRequest.Domain = &rootDomain
	log.Println("listing records of root domain")

	recordListResponse, err := client.DescribeRecordList(recordListRequest)
	if err != nil {
		panic(err)
	}
	needCreating := true
	for _, record := range recordListResponse.Response.RecordList {
		if *record.Type == "TXT" && *record.Name == acmeRecordName {
			needCreating = false

			if *record.Value != acmeChallegeValue {
				recordUpdateRequest := dnspod.NewModifyRecordRequest()
				recordUpdateRequest.Domain = &rootDomain
				recordUpdateRequest.RecordId = record.RecordId
				recordUpdateRequest.Value = &acmeChallegeValue
				recordUpdateRequest.RecordLine = &recordLine
				recordUpdateRequest.RecordType = &recordType
				response, err := client.ModifyRecord(recordUpdateRequest)
				if err != nil {
					panic(err)
				}

				log.Println("record updated, request id: ", response.Response.RequestId)
			} else {
				log.Println("record untouched for value is identical")
			}
			break
		}
	}

	if needCreating {
		recordCreateRequest := dnspod.NewCreateRecordRequest()
		recordCreateRequest.Domain = &rootDomain
		recordCreateRequest.SubDomain = &acmeRecordName
		recordCreateRequest.RecordType = &recordType
		recordCreateRequest.Value = &acmeChallegeValue
		recordCreateRequest.RecordLine = &recordLine
		response, err := client.CreateRecord(recordCreateRequest)
		if err != nil {
			panic(err)
		}
		log.Println("record created, request id: ", response.Response.RequestId)
	}

	recordResolved := false
	recordResolutionStart := time.Now()

	for !recordResolved && time.Since(recordResolutionStart) < recordResolutionTimeout {
		if func() bool {
			recordResolutionContext, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()
			values, err := net.DefaultResolver.LookupTXT(recordResolutionContext, acmeChangeDomain)
			if err != nil {
				log.Println("failed to lookup txt record: ", err)
				return false
			}
			for _, value := range values {
				log.Println("record: '", value, "', match: ", value == acmeChallegeValue, "/", recordResolved)
				recordResolved = recordResolved || (value == acmeChallegeValue)
			}
			return recordResolved
		}() {
			break
		}
		time.Sleep(time.Second)
	}

	time.Sleep(extraWait)
}
