package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
)

type JobStatus string

const (
	statusPending JobStatus = "PENDING"
	statusDone    JobStatus = "Done"
)

type AcceptedResponse struct {
	ServerId string
	Task     Task `json:"task"`
}

type Task struct {
	Href string `json:"href"`
	Id   string `json:"id"`
}

type StatusResponse struct {
	TaskData taskData
	Id string
	ServerId string
}
type taskData struct {
	Result string
	Status JobStatus
	Duration string
}

var (
	ServerId string
	fsClient *firestore.Client
)

func worker(id string) {

	now := time.Now().UTC()
	counter := 0
	for {
		time.Sleep(5000 * time.Millisecond)
		dur := time.Since(now)
		log.Println("Running for: ", dur)
		if counter > 10 {
			t := taskData{"We are golden", statusDone, dur.String()}
			fsClient.Doc("tasks/"+id).Set(context.Background(), &t)
			return
		}
		ctx := context.Background()
		docsnap, err := fsClient.Doc("tasks/" + id).Get(ctx)
		if err != nil {
			log.Println(err)
			return
		}
		var t taskData
		docsnap.DataTo(&t)
		t.Duration = dur.String()
		fsClient.Doc("tasks/"+id).Set(context.Background(), &t)
		counter++
	}
}

func longTaskHandler(w http.ResponseWriter, r *http.Request) {
	id := uuid.New().String()
	accepted := AcceptedResponse{ServerId, Task{"/taskstatus?task=" + id, id}}
	t := taskData{"Nothing yet wait for it....", statusPending, "0"}
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
	time.Sleep(5000 * time.Millisecond)
	ctx := context.Background()
	docsnap, err := fsClient.Doc("tasks/" + task).Get(ctx)
	if err != nil {
		log.Println(err)
		w.Write([]byte("Task not found: " + task ))
		return
	}
	var t taskData
	docsnap.DataTo(&t)
	if t.Status == statusDone {
		fsClient.Doc("tasks/" + task).Delete(ctx)
	}
	jobStatus := StatusResponse{t, task, ServerId}
	js, err := json.Marshal(jobStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
	//w.Write([]byte("Running for:" + dur.String() + " " + t.Result ))
}
func main() {
	ServerId = uuid.New().String()
	log.Println("Starting Long John Silver demo " + ServerId)
	httpListenPort := os.Getenv("PORT")
	if httpListenPort == "" {
		httpListenPort = "8080"
	}
	ctx := context.Background()
	var err error
	fsClient, err = firestore.NewClient(ctx, "ljs-cloud-run")
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
