package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/spf13/viper"

	"github.com/doitintl/long_john_silver/types"
)

type config struct {
	Port             string
	ProjectId        string
	WorkerSleepTime  int
	WorkTime         int
	RequestSleepTime int
}

var (
	ServerId string
	fsClient *firestore.Client
	Config   config
)

func init() {

	v := viper.New()
	v.BindEnv("Port", "PORT")
	v.BindEnv("ProjectId", "PROJECT_ID")
	v.BindEnv("WorkerSleepTime", "SLEEP_TIME")    //in seconds
	v.BindEnv("WorkTime", "WORK_TIME")            // in minuets
	v.BindEnv("RequestSleepTime", "REQUEST_TIME") // in seconds
	err := v.Unmarshal(&Config)
	if err != nil {
		log.Println("unable to decode into struct, %v", err)
	}
}
func worker(id string) {
	now := time.Now().UTC()
	counter := 0
	loops := (Config.WorkTime * 60) / Config.WorkerSleepTime
	// Mimic some work
	for {
		time.Sleep(time.Duration(Config.WorkerSleepTime) * time.Second)
		dur := time.Since(now)
		log.Println("Running for: ", dur)
		// We are done working :)
		if counter >= int(loops) {
			t := types.TaskData{"We are golden", types.StatusDone, dur.String(), "None of your business"}
			fsClient.Doc("tasks/"+id).Set(context.Background(), &t)
			return
		}
		ctx := context.Background()
		docsnap, err := fsClient.Doc("tasks/" + id).Get(ctx)
		if err != nil {
			log.Println(err)
			return
		}
		var t types.TaskData
		docsnap.DataTo(&t)
		t.Duration = dur.String()
		fsClient.Doc("tasks/"+id).Set(context.Background(), &t)
		counter++
	}
}

func longTaskHandler(w http.ResponseWriter, r *http.Request) {

	id := uuid.New().String()
	accepted := types.AcceptedResponse{ServerId, types.Task{"/taskstatus?task=" + id, id}}
	t := types.TaskData{"Nothing yet wait for it....", types.StatusPending, "0", ServerId}
	_, err := fsClient.Doc("tasks/"+id).Create(context.Background(), &t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	js, err := json.Marshal(accepted)
	go worker(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write(js)
}
func taskStatusHandler(w http.ResponseWriter, r *http.Request) {
	task := r.URL.Query().Get("task")
	//throttl a bit
	time.Sleep(time.Duration(time.Duration(Config.RequestSleepTime) * time.Second))
	ctx := context.Background()
	docsnap, err := fsClient.Doc("tasks/" + task).Get(ctx)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Task not found: " + task))
		return
	}
	var t types.TaskData
	docsnap.DataTo(&t)
	if t.Status == types.StatusDone {
		fsClient.Doc("tasks/" + task).Delete(ctx)
	}
	jobStatus := types.StatusResponse{t, task, ServerId}
	js, err := json.Marshal(jobStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
}
func main() {
	ServerId = uuid.New().String()
	log.Println("Starting Long John Silver demo " + ServerId)
	httpListenPort := Config.Port
	if httpListenPort == "" {
		httpListenPort = "8080"
	}
	ctx := context.Background()
	var err error
	fsClient, err = firestore.NewClient(ctx, Config.ProjectId)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	hostPort := net.JoinHostPort("0.0.0.0", httpListenPort)
	mux := http.NewServeMux()
	longtask := http.HandlerFunc(longTaskHandler)
	statustask := http.HandlerFunc(taskStatusHandler)
	mux.Handle("/longtask", longtask)
	mux.Handle("/taskstatus", statustask)
	s := &http.Server{
		Addr:    hostPort,
		Handler: mux,
	}

	if err := s.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
