// Ten plik został przeniesiony z Pythona z pomocą ChatGPT. Działa, więc traktujemy go jako gotowy moduł pogodowy.

package weather

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed wmo_codes.json
var wmoCodesJSON string

// Position reprezentuje współrzędne geograficzne.
type Position struct {
	Latitude  float64
	Longitude float64
}

// WeatherInfo przechowuje ikonę i opis pogody.
type WeatherInfo struct {
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

// WmoCode przechowuje wariant dzienny i nocny opisu pogody.
type WmoCode struct {
	Day   WeatherInfo `json:"day"`
	Night WeatherInfo `json:"night"`
}

// wmoCodes jest ładowane z pliku JSON podczas inicjalizacji.
var wmoCodes map[string]WmoCode

var polishWeatherDescriptions = map[string]string{
	"Sunny":                         "Słonecznie",
	"Clear":                         "Bezchmurnie",
	"Mainly Sunny":                  "Przeważnie słonecznie",
	"Mainly Clear":                  "Przeważnie bezchmurnie",
	"Partly Cloudy":                 "Częściowe zachmurzenie",
	"Cloudy":                        "Pochmurno",
	"Foggy":                         "Mgła",
	"Rime Fog":                      "Mgła osadzająca szadź",
	"Light Drizzle":                 "Lekka mżawka",
	"Drizzle":                       "Mżawka",
	"Heavy Drizzle":                 "Silna mżawka",
	"Light Freezing Drizzle":        "Lekka marznąca mżawka",
	"Freezing Drizzle":              "Marznąca mżawka",
	"Light Rain":                    "Lekki deszcz",
	"Rain":                          "Deszcz",
	"Heavy Rain":                    "Silny deszcz",
	"Light Freezing Rain":           "Lekki marznący deszcz",
	"Freezing Rain":                 "Marznący deszcz",
	"Light Snow":                    "Lekki śnieg",
	"Snow":                          "Śnieg",
	"Heavy Snow":                    "Intensywny śnieg",
	"Snow Grains":                   "Ziarnisty śnieg",
	"Light Showers":                 "Lekkie przelotne opady",
	"Showers":                       "Przelotne opady",
	"Heavy Showers":                 "Silne przelotne opady",
	"Light Snow Showers":            "Lekkie przelotne opady śniegu",
	"Snow Showers":                  "Przelotne opady śniegu",
	"Thunderstorm":                  "Burza",
	"Light Thunderstorms With Hail": "Lekka burza z gradem",
	"Thunderstorm With Hail":        "Burza z gradem",
}

var localityCache = make(map[string]string)
var localityCacheMu sync.Mutex

func init() {
	// Załaduj wmo_codes.json.
	err := json.Unmarshal([]byte(wmoCodesJSON), &wmoCodes)
	if err != nil {
		log.Printf("Błąd parsowania wmo_codes.json: %v", err)
		wmoCodes = make(map[string]WmoCode)
	}
}

// friendlyDate formatuje datę po polsku.
func friendlyDate(t time.Time) string {
	weekdays := []string{"niedz.", "pon.", "wt.", "śr.", "czw.", "pt.", "sob."}
	months := []string{"sty", "lut", "mar", "kwi", "maj", "cze", "lip", "sie", "wrz", "paź", "lis", "gru"}
	return fmt.Sprintf("%s %d %s", weekdays[t.Weekday()], t.Day(), months[int(t.Month())-1])
}

// windDirection zamienia liczbowy kierunek wiatru na strzałkę.
func windDirection(direction float64) string {
	switch {
	case direction >= 0 && direction < 22.5:
		return "↓"
	case direction >= 22.5 && direction < 67.5:
		return "↙"
	case direction >= 67.5 && direction < 112.5:
		return "←"
	case direction >= 112.5 && direction < 157.5:
		return "↖"
	case direction >= 157.5 && direction < 202.5:
		return "↑"
	case direction >= 202.5 && direction < 247.5:
		return "↗"
	case direction >= 247.5 && direction < 292.5:
		return "→"
	case direction >= 292.5 && direction < 337.5:
		return "↘"
	case direction >= 337.5 && direction < 360:
		return "↓"
	default:
		return ""
	}
}

func translateWeatherDescription(description string) string {
	if translated, ok := polishWeatherDescriptions[description]; ok {
		return translated
	}
	return description
}

// FetchLocality pobiera nazwę miejscowości dla podanych współrzędnych.
func FetchLocality(position Position) (string, error) {
	cacheKey := fmt.Sprintf("%.3f,%.3f", position.Latitude, position.Longitude)
	localityCacheMu.Lock()
	if locality, ok := localityCache[cacheKey]; ok {
		localityCacheMu.Unlock()
		return locality, nil
	}
	localityCacheMu.Unlock()

	params := url.Values{}
	params.Set("format", "jsonv2")
	params.Set("lat", fmt.Sprintf("%f", position.Latitude))
	params.Set("lon", fmt.Sprintf("%f", position.Longitude))
	params.Set("addressdetails", "1")
	params.Set("accept-language", "pl,en")
	params.Set("zoom", "10")

	fullURL := "https://nominatim.openstreetmap.org/reverse?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "meshbot/1.0 (Meshtastic bot)")

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("nie udało się połączyć z Nominatim: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		DisplayName string            `json:"display_name"`
		Address     map[string]string `json:"address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	for _, key := range []string{"city", "town", "village", "municipality", "hamlet", "suburb", "county", "state", "country"} {
		if value := strings.TrimSpace(result.Address[key]); value != "" {
			localityCacheMu.Lock()
			localityCache[cacheKey] = value
			localityCacheMu.Unlock()
			return value, nil
		}
	}
	if result.DisplayName != "" {
		parts := strings.Split(result.DisplayName, ",")
		locality := strings.TrimSpace(parts[0])
		localityCacheMu.Lock()
		localityCache[cacheKey] = locality
		localityCacheMu.Unlock()
		return locality, nil
	}
	return "", errors.New("brak nazwy miejscowości dla podanych współrzędnych")
}

// FetchWeather pobiera aktualną pogodę dla podanej pozycji.
func FetchWeather(position Position) (string, error) {
	baseURL := "https://api.open-meteo.com/v1/forecast"
	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%f", position.Latitude))
	params.Set("longitude", fmt.Sprintf("%f", position.Longitude))

	// Dodaj parametry aktualnej pogody.
	for _, p := range []string{
		"temperature_2m",
		"is_day",
		"precipitation",
		"weather_code",
		"wind_speed_10m",
		"wind_direction_10m",
	} {
		params.Add("current", p)
	}

	fullURL := baseURL + "?" + params.Encode()
	resp, err := http.Get(fullURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("nie udało się połączyć z serwerem Open-Meteo: %d - %s", resp.StatusCode, string(body))
	}

	var weather map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		return "", err
	}

	current, ok := weather["current"].(map[string]interface{})
	if !ok {
		return "", errors.New("brak aktualnych danych pogodowych")
	}

	// Pobierz kod pogody i wybierz wariant dzienny albo nocny.
	codeVal := current["weather_code"]
	codeStr := ""
	switch v := codeVal.(type) {
	case float64:
		codeStr = strconv.Itoa(int(v))
	case string:
		codeStr = v
	}

	isDay := 1
	if v, ok := current["is_day"].(float64); ok {
		isDay = int(v)
	}

	var weatherInfo WeatherInfo
	if code, found := wmoCodes[codeStr]; found {
		if isDay == 1 {
			weatherInfo = code.Day
		} else {
			weatherInfo = code.Night
		}
	}

	icon := weatherInfo.Icon
	description := translateWeatherDescription(weatherInfo.Description)
	temp := fmt.Sprintf("%v", current["temperature_2m"])

	// Pobierz jednostki.
	currentUnits, _ := weather["current_units"].(map[string]interface{})
	tempUnit := ""
	if currentUnits != nil {
		if v, ok := currentUnits["temperature_2m"].(string); ok {
			tempUnit = v
		}
	}
	precip := fmt.Sprintf("%v", current["precipitation"])
	precipUnit := ""
	if currentUnits != nil {
		if v, ok := currentUnits["precipitation"].(string); ok {
			precipUnit = v
		}
	}
	windSpeed := fmt.Sprintf("%v", current["wind_speed_10m"])
	windSpeedUnit := ""
	if currentUnits != nil {
		if v, ok := currentUnits["wind_speed_10m"].(string); ok {
			windSpeedUnit = v
		}
	}

	windDirFloat := 0.0
	if v, ok := current["wind_direction_10m"].(float64); ok {
		windDirFloat = v
	}
	windDir := windDirection(windDirFloat)

	// Sformatuj wynik.
	result := fmt.Sprintf(
		"🌡️ %s%s %s %s\n💧 %s%s 🌬️ %s%s %s",
		temp, tempUnit,
		icon, description,
		precip, precipUnit,
		windSpeed, windSpeedUnit, windDir,
	)
	return result, nil
}

// FetchForecast pobiera prognozę pogody dla podanej pozycji.
func FetchForecast(position Position) (string, error) {
	baseURL := "https://api.open-meteo.com/v1/forecast"
	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%f", position.Latitude))
	params.Set("longitude", fmt.Sprintf("%f", position.Longitude))

	// Dodaj parametry prognozy dziennej.
	for _, p := range []string{
		"weather_code",
		"temperature_2m_max",
		"temperature_2m_min",
		"precipitation_sum",
		"precipitation_probability_max",
		"wind_speed_10m_max",
		"wind_direction_10m_dominant",
	} {
		params.Add("daily", p)
	}
	params.Set("timezone", "auto")

	fullURL := baseURL + "?" + params.Encode()
	resp, err := http.Get(fullURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return "", fmt.Errorf("nie udało się połączyć z serwerem Open-Meteo: %d - %s", resp.StatusCode, string(body))
	}

	var forecast map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&forecast); err != nil {
		return "", err
	}

	daily, ok := forecast["daily"].(map[string]interface{})
	if !ok {
		return "", errors.New("brak dziennych danych prognozy")
	}

	units, _ := forecast["daily_units"].(map[string]interface{})

	// Utwórz listę map, po jednej na dzień, z odpowiedzi API.
	timeArr, ok := daily["time"].([]interface{})
	if !ok {
		return "", errors.New("nie znaleziono dziennych danych czasu")
	}
	n := len(timeArr)
	structuredForecast := make([]map[string]string, n)
	for i := 0; i < n; i++ {
		structuredForecast[i] = make(map[string]string)
	}

	// Przepisz dzienne dane do prostszej struktury.
	for key, val := range daily {
		arr, ok := val.([]interface{})
		if !ok {
			continue
		}
		newKey := key
		if key == "time" {
			newKey = "day"
		}
		if key == "weather_code" {
			newKey = "icon"
		}
		for i, v := range arr {
			var valueStr string
			if newKey == "day" {
				// Sparsuj datę i sformatuj ją po polsku.
				if dateStr, ok := v.(string); ok {
					t, err := time.Parse("2006-01-02", dateStr)
					if err == nil {
						valueStr = friendlyDate(t)
					} else {
						valueStr = dateStr
					}
				}
			} else if newKey == "icon" {
				// Znajdź kod pogody i ustaw ikonę oraz opis.
				codeStr := ""
				switch cv := v.(type) {
				case float64:
					codeStr = strconv.Itoa(int(cv))
				case string:
					codeStr = cv
				}
				if code, found := wmoCodes[codeStr]; found {
					valueStr = code.Day.Icon
					structuredForecast[i]["description"] = translateWeatherDescription(code.Day.Description)
				}
			} else if key == "wind_direction_10m_dominant" {
				// Zamień liczbowy kierunek wiatru na strzałkę.
				if dir, ok := v.(float64); ok {
					valueStr = windDirection(dir)
				}
			} else {
				// Dodaj jednostkę, jeśli jest dostępna.
				unit := ""
				if units != nil {
					if u, ok := units[key].(string); ok {
						unit = u
					}
				}
				valueStr = fmt.Sprintf("%v%s", v, unit)
			}
			structuredForecast[i][newKey] = valueStr
		}
	}

	// Zbuduj tekst prognozy, maksymalnie dla 2 dni.
	forecastStr := ""
	limit := 2
	if n < limit {
		limit = n
	}
	for i := 0; i < limit; i++ {
		day := structuredForecast[i]
		forecastStr += fmt.Sprintf("%s: 🌡️ %s/%s %s %s 💧%s %s 🌬️%s %s\n",
			day["day"],
			day["temperature_2m_max"],
			day["temperature_2m_min"],
			day["icon"],
			day["description"],
			day["precipitation_sum"],
			day["precipitation_probability_max"],
			day["wind_speed_10m_max"],
			day["wind_direction_10m_dominant"],
		)
	}

	return forecastStr, nil
}
