#!/bin/bash

curl -X POST https://www.strava.com/oauth/token \
	-F client_id= \
	-F client_secret= \
	-F code= \
	-F grant_type=authorization_code

	# http://www.strava.com/oauth/authorize?client_id=&response_type=code&redirect_uri=http://localhost/exchange_token&approval_prompt=force&scope=read