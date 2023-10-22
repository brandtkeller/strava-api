# Strava API Access

## Create API Access to get tokens and clientID
Follow the steps outlined [here](https://towardsdatascience.com/using-the-strava-api-and-pandas-to-explore-your-activity-data-d94901d9bfde) to create API access and get clientID and clientSecret.

## One time manual authorization - replace client_id

Replace `client_Id` with your own and use your browser for a one-time authentication to get a code.

https://www.strava.com/oauth/authorize?client_id=your_client_id&redirect_uri=http://localhost&response_type=code&scope=activity:read_all

## Grab code from the url - seeing an error is okay

http://localhost/?state=&code=<yourcodehere>&scope=read,activity:read_all

## Create environment file 

- Create a file called `strava.env` in the root of the project with the following contents
```
STRAVA_CLIENT_ID=<clientId>
STRAVA_CLIENT_SECRET=<clientsecret>
STRAVA_REFRESH_TOKEN=<refreshToken>
```

## Run the app
`go run main.go`