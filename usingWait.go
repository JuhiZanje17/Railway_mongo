package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	mongoURL = "mongodb://localhost:27017"
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
	trainCollection := client.Database("railway_data").Collection("trains")
	return trainCollection, client
}

func insertData(wg *sync.WaitGroup) {

	trainCollection, client := getCollection()
	defer client.Disconnect(context.TODO())

	lines, err := ReadCsv("Indian_railway1.csv")
	if err != nil {
		panic(err)
	}
	count := 0

	// Loop through lines & turn into object
	for _, line := range lines {
		wg.Add(1)
		count++

		go func() {
			defer wg.Done()
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
		}()
		// break
		wg.Wait()

	}
	fmt.Println("Done", count)

}

func main() {

	var wg sync.WaitGroup
	start := time.Now()
	insertData(&wg)

	elapsed := time.Since(start)
	log.Printf("Time taken %s", elapsed)

	fs := http.FileServer(http.Dir("static/"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/fetch", fetchFun)

	http.ListenAndServe(":8080", nil)

}

func fetchFun(w http.ResponseWriter, r *http.Request) {

	collection, client := getCollection()
	defer client.Disconnect(context.TODO())
	//Define filter query for fetching specific document from collection
	filter := bson.D{{}} //bson.D{{}} specifies 'all documents'
	issues := []Train{}
	//Perform Find operation & validate against the error.
	cur, findError := collection.Find(context.TODO(), filter)
	if findError != nil {
		panic(findError)
	}
	//Map result to slice
	for cur.Next(context.TODO()) {
		t := Train{}
		err := cur.Decode(&t)
		if err != nil {
			panic(err)
		}
		//fmt.Println(t)
		issues = append(issues, t)
	}
	// once exhausted, close the cursor
	cur.Close(context.TODO())
	if len(issues) == 0 {
		panic(mongo.ErrNoDocuments)
	}
	//fmt.Println(issues)

	res, _ := json.Marshal(issues)
	w.Write(res)

}
