package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	channelCapacity = 20
)

var (
	ch = make(chan bool, channelCapacity)
)

type Train struct {
	TrainNo                string `bson:"TrainNo"`
	TrainName              string `bson:"TrainName"`
	SEQ                    string `bson:"SEQ"`
	StationCode            string `bson:"StationCode"`
	StationName            string `bson:"StationName"`
	ArrivalTime            string `bson:"ArrivalTime"`
	DepartureTime          string `bson:"DepartureTime"`
	Distance               string `bson:"Distance"`
	SourcetSation          string `bson:"SourcetSation"`
	SourceStationName      string `bson:"SourceStationName"`
	DestinationStation     string `bson:"DestinationStation"`
	DestinationStationName string `bson:"DestinationStationName"`
}

func getCredentials(key string) string {

	// load .env file
	err := godotenv.Load("../Cred.env")

	if err != nil {
		log.Fatalf("Error loading .env file", err)
	}

	return os.Getenv(key)
}

// ReadCsv accepts a file and returns its content as a multi-dimentional type
// with lines and each column. Only parses to string type.
func ReadCsv(filename string) ([][]string, error) {

	// Open CSV file
	f, err := os.Open(filename)
	if err != nil {
		return [][]string{}, err
	}
	defer f.Close()

	// Read File into a Variable
	lines, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return [][]string{}, err
	}

	return lines, nil
}

func getCollection() (*mongo.Collection, *mongo.Client) {

	mongoURL := getCredentials("MONGO_URL")
	db, collection := getCredentials("DB"), getCredentials("COLLECTION")
	// Set client options
	clientOptions := options.Client().ApplyURI(mongoURL)

	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), clientOptions)

	if err != nil {
		log.Fatal(err)
	}

	// Check the connection
	err = client.Ping(context.TODO(), nil)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB!")
	trainCollection := client.Database(db).Collection(collection)
	return trainCollection, client
}

func insertData() {

	trainCollection, client := getCollection()
	defer client.Disconnect(context.TODO())

	lines, err := ReadCsv("Indian_railway1.csv")
	if err != nil {
		fmt.Println(err)
	}
	count := 0

	// Loop through lines & turn into object
	for _, line := range lines {
		ch <- true
		count++

		go func() {

			data := Train{
				TrainNo:                line[0],
				TrainName:              line[1],
				SEQ:                    line[2],
				StationCode:            line[3],
				StationName:            line[4],
				ArrivalTime:            line[5],
				DepartureTime:          line[6],
				Distance:               line[7],
				SourcetSation:          line[8],
				SourceStationName:      line[9],
				DestinationStation:     line[10],
				DestinationStationName: line[11],
			}

			_, err := trainCollection.InsertOne(context.TODO(), data)
			if err != nil {
				log.Fatal(err)
			}
			<-ch
		}()
		// break

	}
	for i := 0; i < channelCapacity; i++ {
		ch <- true
	}
	fmt.Println("Done", count)

}

func fetchFun(w http.ResponseWriter, r *http.Request) {

	collection, client := getCollection()
	defer client.Disconnect(context.TODO())
	//Define filter query for fetching specific document from collection
	filter := bson.D{{}} //bson.D{{}} specifies 'all documents'
	issues := []Train{}

	param, ok := r.URL.Query()["page"]
	if !ok {
		fmt.Println("Error occurred")
	}
	page, _ := strconv.Atoi(param[0])
	fmt.Println(page)

	option := options.Find()
	option.SetLimit(15)
	option.SetSkip(int64(page * 10))

	//Perform Find operation & validate against the error.
	cur, findError := collection.Find(context.TODO(), filter, option)
	if findError != nil {
		fmt.Println(findError)
	}
	//Map result to slice
	for cur.Next(context.TODO()) {
		t := Train{}
		err := cur.Decode(&t)
		if err != nil {
			fmt.Println(err)
		}
		//fmt.Println(t)
		issues = append(issues, t)
	}
	// once exhausted, close the cursor
	cur.Close(context.TODO())
	if len(issues) == 0 {
		fmt.Println(mongo.ErrNoDocuments)
	}
	//fmt.Println(issues)

	res, _ := json.Marshal(issues)
	w.Write(res)

}

func main() {

	boolInsert := flag.Bool("insert", false, "a bool")
	flag.Parse()

	if *boolInsert {
		insertData()
	} else {
		fmt.Println("not inserted")
	}

	start := time.Now()

	elapsed := time.Since(start)
	log.Printf("Time taken %s", elapsed)

	fs := http.FileServer(http.Dir("static/"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/fetch", fetchFun)

	http.ListenAndServe(":8080", nil)

}
