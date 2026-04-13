// 📍 FILE: services/weather.go

package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TODO: Move this to a .env file or config file later
const (
	tomorrowAPIKey = "VRe45t8MipOS1OhCGBg6ACwwUDu91b8k"
	tomorrowBaseURL = "https://api.tomorrow.io/v4/weather/forecast"
)

// This struct is a small, clean version for our app.
// We only parse the fields the UI actually needs.
type AppWeather struct {
	Temperature      float64 `json:"temperature"`
	Humidity         float64 `json:"humidity"`
	Precipitation    float64 `json:"precipitation"`
	WindSpeed        float64 `json:"windSpeed"`
	WeatherCode      int     `json:"weatherCode"`
	WeatherCondition string  `json:"weatherCondition"`
}

// This is the massive struct to catch the full Tomorrow.io response
// We use this so json.Unmarshal doesn't fail.
type tomorrowResponse struct {
	Timelines struct {
		Hourly []struct {
			Time   time.Time `json:"time"`
			Values struct {
				Temperature      float64 `json:"temperature"`
				Humidity         float64 `json:"humidity"`
				Precipitation    float64 `json:"precipitationIntensity"` // Note the name change
				WindSpeed        float64 `json:"windSpeed"`
				WeatherCode      int     `json:"weatherCode"`
			} `json:"values"`
		} `json:"hourly"`
	} `json:"timelines"`
}

// GetWeatherForLocation calls the Tomorrow.io API
func GetWeatherForLocation(lat string, lon string) (*AppWeather, error) {
	// 1. Build the request
	url := fmt.Sprintf("%s?location=%s,%s&apikey=%s&units=metric", tomorrowBaseURL, lat, lon, tomorrowAPIKey)

	fmt.Println("--- DEBUG: Attempting to use Tomorrow.io API key:", tomorrowAPIKey)
	fmt.Println("--- DEBUG: Calling full URL:", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Add("accept", "application/json")

	// 2. Execute the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Tomorrow.io: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Tomorrow.io API error (%d): %s", resp.StatusCode, string(body))
	}

	// 3. Parse the full JSON response
	var apiResponse tomorrowResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// 4. Check if we have at least one hourly forecast
	if len(apiResponse.Timelines.Hourly) == 0 {
		return nil, fmt.Errorf("no hourly data returned from API")
	}

	// 5. Map the first hourly forecast (the "current" weather) to our clean struct
	currentWeather := apiResponse.Timelines.Hourly[0].Values
	appWeather := &AppWeather{
		Temperature:      currentWeather.Temperature,
		Humidity:         currentWeather.Humidity,
		Precipitation:    currentWeather.Precipitation,
		WindSpeed:        currentWeather.WindSpeed,
		WeatherCode:      currentWeather.WeatherCode,
		WeatherCondition: mapWeatherCodeToString(currentWeather.WeatherCode), // Helper function
	}

	return appWeather, nil
}

// mapWeatherCodeToString is a simple helper to get a description
// (Based on Tomorrow.io documentation)
func mapWeatherCodeToString(code int) string {
	switch code {
	case 1000:
		return "Clear"
	case 1100, 1101, 1102:
		return "Partly Cloudy"
	case 1001:
		return "Cloudy"
	case 2000, 2100:
		return "Fog"
	case 4000, 4001, 4200, 4201:
		return "Rain"
	case 5000, 5001, 5100, 5101:
		return "Snow"
	case 6000, 6001, 6200, 6201:
		return "Freezing Rain"
	case 7000, 7101, 7102:
		return "Ice Pellets"
	case 8000:
		return "Thunderstorm"
	default:
		return "Unknown"
	}
}