#!/bin/bash

curl -X POST https://www.strava.com/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=115159" \
  -d "client_secret=aebb6ea31374a00738459193244eed9bdac64b66" \
  -d "code=ff3fd9a94c01a74306e1c24d2bd18d9053a08c84" \
  -d "grant_type=authorization_code"

	# http://www.strava.com/oauth/authorize?client_id=&response_type=code&redirect_uri=http://localhost/exchange_token&approval_prompt=force&scope=read

	# http://localhost/exchange_token?state=&code=ff3fd9a94c01a74306e1c24d2bd18d9053a08c84&scope=read,activity:read_all