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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	channelCapacity = 20
)

var (
	ch                         = make(chan bool, channelCapacity)
	db, dbCollection, mongoURL = getCredentials("DB"), getCredentials("COLLECTION"), getCredentials("MONGO_URL")
)

type Train struct {
	TrainNo                string `bson:"TrainNo"`
	TrainName              string `bson:"TrainName"`
	SEQ                    int    `bson:"SEQ"`
	StationCode            string `bson:"StationCode"`
	StationName            string `bson:"StationName"`
	ArrivalTime            string `bson:"ArrivalTime"`
	DepartureTime          string `bson:"DepartureTime"`
	Distance               int    `bson:"Distance"`
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
	trainCollection := client.Database(db).Collection(dbCollection)
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
		seq, _ := strconv.Atoi(line[2])
		dis, _ := strconv.Atoi(line[7])

		go func(line []string) {

			data := Train{
				TrainNo:                line[0],
				TrainName:              line[1],
				SEQ:                    seq,
				StationCode:            line[3],
				StationName:            line[4],
				ArrivalTime:            line[5],
				DepartureTime:          line[6],
				Distance:               dis,
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
		}(line)

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
	//fmt.Println(page)

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

func searchFun(w http.ResponseWriter, r *http.Request) {

	collection, client := getCollection()
	defer client.Disconnect(context.TODO())
	var query primitive.D

	if trainNo, ok := r.URL.Query()["tNo"]; ok {
		fmt.Println(trainNo[0])
		query = append(query, bson.E{"TrainNo", trainNo[0]})
	}

	if arrivalTime, ok := r.URL.Query()["aTime"]; ok {
		query = append(query, bson.E{"ArrivalTime", arrivalTime[0]})
	}

	if departureTime, ok := r.URL.Query()["dTime"]; ok {
		query = append(query, bson.E{"DepartureTime", departureTime[0]})
	}

	if stationName, ok := r.URL.Query()["sName"]; ok {
		query = append(query, bson.E{"StationName", stationName[0]})
	}

	filterCursor, err := collection.Find(
		context.Background(), query)

	var trainsFiltered []bson.M
	if err = filterCursor.All(context.TODO(), &trainsFiltered); err != nil {
		log.Fatal(err)
	}

	res, _ := json.Marshal(trainsFiltered)
	w.Write(res)
}

func searchDistFun(w http.ResponseWriter, r *http.Request) {

	collection, client := getCollection()
	defer client.Disconnect(context.TODO())

	sName, _ := r.URL.Query()["sName"]
	dName, _ := r.URL.Query()["dName"]

	filterCursor, err := collection.Find(
		context.Background(), bson.M{"StationName": sName[0]})

	var trainsFiltered []bson.M
	if err = filterCursor.All(context.TODO(), &trainsFiltered); err != nil {
		log.Fatal(err)
	}

	var trainsFiltered2 []bson.M
	var minTrain, maxTrain bson.M
	var minDistance, maxDistance int32
	maxDistance, minDistance = -1, -1

	for _, v := range trainsFiltered {

		tNo := v["TrainNo"].(string)
		seq := v["SEQ"].(int32)
		sDist := v["Distance"].(int32)

		filterCursor2, err := collection.Find(
			context.TODO(), bson.D{{"TrainNo", tNo}, {"StationName", dName[0]}}) //{"SEQ", bson.E{"$gt", num}}

		var temp bson.M

		for filterCursor2.Next(context.TODO()) {
			if err = filterCursor2.Decode(&temp); err != nil {
				log.Fatal(err)
			}
			if (temp["SEQ"].(int32)) > seq {
				curDist := (temp["Distance"].(int32)) - sDist
				if minDistance < 0 || minDistance > curDist {
					minDistance = curDist
					minTrain = temp
				}

				if maxDistance < curDist {
					maxDistance = curDist
					maxTrain = temp
				}

			}
		}

	}

	trainsFiltered2 = append(trainsFiltered2, minTrain)
	trainsFiltered2 = append(trainsFiltered2, maxTrain)

	resFinal, _ := json.Marshal(trainsFiltered2)
	w.Write(resFinal)

}

func sortDistFunc(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	st := r.URL.Query().Get("sName")
	dtn := r.URL.Query().Get("dName")

	collection, client := getCollection()
	defer client.Disconnect(context.TODO())

	lookupStage := bson.D{{"$lookup", bson.D{{"from", dbCollection}, {"localField", "TrainNo"}, {"foreignField", "TrainNo"}, {"as", "trainData"}}}}
	unwindStage := bson.D{{"$unwind", bson.D{{"path", "$trainData"}, {"preserveNullAndEmptyArrays", false}}}}
	filterStage := bson.D{{"$match", bson.D{{"StationName", st}, {"trainData.StationName", dtn}}}}
	// nextfilterStage := bson.D{{"$match", bson.D{{"seq", bson.D{{"$gt", "trainData.seq"}}}}}}
	fmt.Println("hi")

	showLoadedCursor, _ := collection.Aggregate(context.TODO(), mongo.Pipeline{lookupStage, unwindStage, filterStage})

	var showsLoaded []bson.M

	count := 0
	for showLoadedCursor.Next(context.TODO()) {
		var t bson.M
		err := showLoadedCursor.Decode(&t)
		if err != nil {
			panic(err)
		}
		seq := t["SEQ"].(int32)
		dseq := t["trainData"].(bson.M)["SEQ"].(int32)

		// fmt.Println(seq, dseq)
		if seq < dseq {
			count++

			dt := strings.ReplaceAll(t["DepartureTime"].(string), ":", "")                     //departure time from soure
			at := strings.ReplaceAll(t["trainData"].(bson.M)["ArrivalTime"].(string), ":", "") //arrival time at destination

			dtime, err := time.Parse("150405", dt)
			if err != nil {
				log.Fatal(err)
			}
			// fmt.Println(dtime)

			atime, err := time.Parse("150405", at)
			if err != nil {
				log.Fatal(err)
			}
			// fmt.Println(atime)

			t["DeptTime"] = dtime
			t["ArriTime"] = atime

			diff := atime.Sub(dtime)
			// if(diff < 0) {
			// 	diff = dtime.Sub(atime)
			// }
			// fmt.Println(diff)
			out := time.Time{}.Add(diff).String()
			t["timeTaken"] = out[11:19]
			showsLoaded = append(showsLoaded, t)
		}
	}

	sort.Slice(showsLoaded, func(i, j int) bool {

		dtime, err := time.Parse("150405", strings.ReplaceAll(showsLoaded[i]["timeTaken"].(string), ":", ""))
		if err != nil {
			log.Fatal(err)
		}
		// fmt.Println(dtime)

		atime, err := time.Parse("150405", strings.ReplaceAll(showsLoaded[j]["timeTaken"].(string), ":", ""))
		if err != nil {
			log.Fatal(err)
		}
		diff := dtime.Sub(atime)
		if diff < 0 {
			return true
		}
		return false
	})

	res, _ := json.Marshal(struct {
		IsError bool
		Msg     string
		Count   int
		Data    []bson.M
	}{false, "Data Fetched Successfully", count, showsLoaded})
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

	http.HandleFunc("/search", searchFun)
	http.HandleFunc("/searchDist", searchDistFun)
	http.HandleFunc("/sortDist", sortDistFunc)

	http.ListenAndServe(":8080", nil)

}
