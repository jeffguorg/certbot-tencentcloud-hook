#!/usr/bin/env bash

export AUTHHOOK_TIMEOUT={{ .timeout }}
export AUTHHOOK_EXTRA_WAIT={{ .extraWait }}

export AUTHHOOK_RECORD_LINE={{ .recordLine }}
export AUTHHOOK_RECORD_TYPE={{ .recordType }}

export AUTHHOOK_ROOT_DOMAIN={{ .rootDomain }}
export AUTHHOOK_SECRET_ID={{ .secretId }}
export AUTHHOOK_SECRET_KEY={{ .secretKey }}

{{ .self }}
