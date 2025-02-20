package main

import (
	"encoding/json"
	// "fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type authResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
}

type activity struct {
	Id          int     `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Distance    float64 `json:"distance"`
	MovingTime  int     `json:"moving_time"`
	ElapsedTime int     `json:"elapsed_time"`
	Type        string  `json:"type"`
	StartDate   string  `json:"start_date"`
	StartTime   string  `json:"start_time"`
	EndDate     string  `json:"end_date"`
	EndTime     string  `json:"end_time"`
}

type envVars struct {
	StravaClientId     string `mapstructure:"STRAVA_CLIENT_ID"`
	StravaClientSecret string `mapstructure:"STRAVA_CLIENT_SECRET"`
	StravaRefreshToken string `mapstructure:"STRAVA_REFRESH_TOKEN"`
	StravaAccessToken  string `mapstructure:"STRAVA_ACCESS_TOKEN"`
}

type historicalData struct {
}

func (hd *historicalData) GetData() (int, error) {
	return 0, nil
}

func (hd *historicalData) StoreData(year int, month time.Month, distance float64) error {
	return nil
}

func main() {

	// setup logging
	logger := log.Default()

	var config envVars
	// Load environment configuration - IE secret tokens
	viper.SetConfigName("strava")
	viper.AddConfigPath(".")
	viper.SetConfigType("env")

	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		logger.Fatal(err)
	}

	if err := viper.Unmarshal(&config); err != nil {
		logger.Fatal(err)
	}

	// Create HTTP Client
	client := http.Client{}

	// authUrl := "https://www.strava.com/oauth/token"
	activitesUrl := "https://www.strava.com/api/v3/athlete/activities"

	// // Authenticate to get access token
	// authInfo := fmt.Sprintf("client_id=%s&grant_type=refresh_token&refresh_token=%s&f=json", config.StravaClientId, config.StravaRefreshToken)
	// req, err := http.NewRequest("POST", authUrl, strings.NewReader(authInfo))
	// if err != nil {
	// 	//Handle Error
	// 	logger.Fatal(err)
	// }

	// q := req.URL.Query()
	// q.Add("client_id", config.StravaClientId)
	// q.Add("client_secret", config.StravaClientSecret)
	// q.Add("refresh_token", config.StravaRefreshToken)
	// q.Add("grant_type", "refresh_token")
	// q.Add("f", "json")
	// req.URL.RawQuery = q.Encode()

	// res, err := client.Do(req)
	// if err != nil {
	// 	//Handle Error
	// 	logger.Fatal(err)
	// }

	// body, readErr := io.ReadAll(res.Body)
	// if readErr != nil {
	// 	logger.Fatal(readErr)
	// }

	// logger.Println(string(body))

	// Unmarshall json response to struct
	result := authResponse{
		AccessToken:  config.StravaAccessToken,
		RefreshToken: config.StravaRefreshToken,
	}
	// if err := json.Unmarshal(body, &result); err != nil { // Parse []byte to go struct pointer
	// 	logger.Println("Can not unmarshal JSON")
	// }

	logger.Println("Authenticated - Preparing to get activities by page of 200")

	// Create a slice of activities to hold all activities
	activities := make([]activity, 0)
	page := 1

	for {
		// Create a placeholder slice of activities for each page of results (200 max)
		pageActivities := make([]activity, 0)
		req, err := http.NewRequest("GET", activitesUrl, nil)
		if err != nil {
			//Handle Error
			logger.Fatal(err)
		}
		q := req.URL.Query()
		q.Add("per_page", "200")
		q.Add("page", strconv.Itoa(page))
		req.URL.RawQuery = q.Encode()

		req.Header = http.Header{
			"Authorization": []string{"Bearer " + result.AccessToken},
		}

		res, err := client.Do(req)
		if err != nil {
			//Handle Error
			logger.Fatal(err)
		}

		body, readErr := io.ReadAll(res.Body)
		if readErr != nil {
			logger.Fatal(readErr)
		}

		logger.Println(string(body))

		if err := json.Unmarshal(body, &pageActivities); err != nil { // Parse []byte to go struct pointer
			logger.Fatal(err)
		}

		if len(pageActivities) == 200 {
			// if we get a total of 200 activities, there may be
			logger.Printf("Page %d retrieved with %d activities\n", page, len(pageActivities))
			page++
			activities = append(activities, pageActivities...)
		} else {
			logger.Printf("Page %d retrieved with %d activities\n", page, len(pageActivities))
			activities = append(activities, pageActivities...)
			break
		}
	}

	// Log total number of activities
	logger.Printf("Total Number of activities: %d\n", len(activities))

	var deskCount int
	var distance float64

	for _, activity := range activities {
		if strings.ToLower(activity.Name) == "desk treadmill" {
			// logger.Printf("Desk Treadmill Activity: %s\n", activity.StartDate)

			// timestamp, err := time.Parse(time.RFC3339, activity.StartDate)
			// if err != nil {
			// 	logger.Fatalf("Error parsing date: %s\n", activity.StartDate)
			// 	return
			// }
			distance += activity.Distance
			deskCount++
		}
	}

	// Log number of desk treadmill activities
	logger.Printf("Desk Treadmill Activities: %d\n", deskCount)
	// Log number of miles after converting meters to miles
	logger.Printf("Total Distance: %f Miles since September 12th \n", distance*0.000621371)

}
