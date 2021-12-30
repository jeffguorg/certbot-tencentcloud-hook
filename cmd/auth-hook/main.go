package main

import (
	"context"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
)

func main() {
	viper.SetDefault("RecordType", "TXT")
	viper.SetDefault("RecordLine", "默认")
	viper.SetDefault("ResolutionTimeout", time.Second*600)
	viper.SetDefault("ExtraWait", time.Second*100)

	viper.SetConfigName("authhook")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc")
	if path, err := os.UserHomeDir(); err == nil {
		viper.AddConfigPath(path)
	}

	viper.SetEnvPrefix("AUTHHOOK")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}

	recordResolutionTimeout := viper.GetDuration("ResolutionTimeout")
	extraWait := viper.GetDuration("ExtraWait")
	recordType := viper.GetString("RecordType")
	recordLine := viper.GetString("RecordLine")

	secretId := viper.GetString("SecretID")
	secretKey := viper.GetString("SecretKey")

	rootDomain := viper.GetString("RootDomain")
	certbotDomain := os.Getenv("CERTBOT_DOMAIN")
	acmeChangeDomain := "_acme-challenge." + certbotDomain
	acmeRecordName := strings.ReplaceAll(acmeChangeDomain, "."+rootDomain, "")

	acmeChallegeValue := os.Getenv("CERTBOT_VALIDATION")

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
			needCreating = false

			log.Println("record updated, request id: ", response.Response.RequestId)
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
