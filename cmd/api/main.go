package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type WeatherResponse struct {
	ResolvedAddress string     `json:"resolvedAddress"`
    Address string             `json:"address"`
	Days            []DayEntry `json:"days"`
}

type DayEntry struct {
	Date       string  `json:"datetime"`
	TempMax    float64 `json:"tempmax"`
	TempMin    float64 `json:"tempmin"`
	Conditions string  `json:"conditions"`
    UVindex float32 `json:"uvindex"`
}

func getEnvVar(key string) string {
    err := godotenv.Load(".env")

    if err != nil {
        log.Fatal(err)
    }
    return os.Getenv(key)
}

func checkCache(client *redis.Client, city string) (WeatherResponse, bool) {
    fmt.Println("1. CHECKING CACHE FOR EXISTING ENTRY")
    val, err := (*client).Get(context.Background(), city).Result()
    if err != nil {
        log.Println(err.Error())
        fmt.Println("2. CACHE RESPONSE: No entry found for " + city)
        return WeatherResponse{}, false
    }
    

    val2 := []byte(val)
    var weather WeatherResponse
    err = json.Unmarshal(val2, &weather)
    if err != nil {
        log.Println(err.Error())
        fmt.Println("2. CACHE RESPONSE: Entry found but failed to unmarshal")
        return WeatherResponse{}, false
    }

    if weather.Address == city {
        fmt.Println("2. CACHE RESPONSE: Entry found for " + city)
        return weather, true
    } else {
        fmt.Println("5. CACHE RESPONSE: Wrong entry found for " + city)
        return WeatherResponse{}, false
    }


    
}


func setCache(client *redis.Client, weather *WeatherResponse) {
    fmt.Println("5. SAVING CACHED RESULTS")
    jsonString, err := json.Marshal(weather)
    if err != nil {
        log.Fatal(err.Error())
        return
    }
    err2 := client.Set(context.Background(), weather.Address, jsonString, 12 * time.Hour).Err()
    if err2 != nil {
        log.Fatal(err2.Error())
        return
    }
}


func main() {    

    city := ""

    client := redis.NewClient(&redis.Options{
        Addr:	  getEnvVar("REDIS_ADDR"),
        Password: getEnvVar("REDIS_PASSWORD"), // No password set
        DB:		  0,  // Use default DB
        //Protocol: 2,  // Connection protocol
    })

    router := gin.Default()
    router.GET("/weather/:city", func(c *gin.Context) {
        city = c.Param("city")



        ping, err := client.Ping(context.Background()).Result()
        if err != nil {
            log.Fatal(err.Error())

        }

        fmt.Println(ping)

        weather, exists := checkCache(client, city)
        if !exists {
            fmt.Println("3. REQUEST WEATHER API")
            dbAuthCode := getEnvVar("DB_AUTH_CODE")
            url := "https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/" + city +
                "?unitGroup=metric&include=days&key=" + dbAuthCode + "&contentType=json"

            resp, err := http.Get(url)
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch weather"})
                return
            }
            
            defer resp.Body.Close()

            body, err := io.ReadAll(resp.Body)
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read response"})
                return
		    }

            err = json.Unmarshal(body, &weather)
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
                return
            }
            fmt.Println("4. WEATHER API RESPONSE SUCCESSFUL")

            setCache(client, &weather)
        }

        // Now we have a working weatherResponse
       

        
		

        
        

        c.HTML(http.StatusOK, "weather.html", gin.H{
            "title":   "Wetter für " + weather.ResolvedAddress,
            "weather": weather,
        })

        // Return the raw JSON response from Visual Crossing
    })
    router.LoadHTMLGlob("templates/*")

    //fmt.Printf("Value retrieved from redis: %s\n", val)

    router.Run("localhost:8080")
}
