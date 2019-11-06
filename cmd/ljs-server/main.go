package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
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
	Quit     chan string
)

func init() {
	Quit = make(chan string)
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
func worker(id string, data string) {
	now := time.Now().UTC()
	counter := 0
	loops := (Config.WorkTime * 60) / Config.WorkerSleepTime
	// Mimic some work
	for {
		select {
		case <-Quit:
			msg := <-Quit
			if msg == id {
				return
			}
			break
		default:
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
}

func longTask(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var p Payload
	err := decoder.Decode(&p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if p.Data == "" {
		http.Error(w, "Missing data in payload", http.StatusBadRequest)
		return
	}
	id := uuid.New().String()
	accepted := types.AcceptedResponse{ServerId, types.Task{"/job/" + id, id}}
	t := types.TaskData{"Nothing yet wait for it....", types.StatusPending, "0", ServerId}
	_, err = fsClient.Doc("tasks/"+id).Create(context.Background(), &t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	js, err := json.Marshal(accepted)
	go worker(id, p.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write(js)
}
func taskStatus(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	task, _ := params["id"]
	time.Sleep(time.Duration(time.Duration(Config.RequestSleepTime) * time.Second))
	ok, t, err := getTask(task)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	jobStatus := types.StatusResponse{t, task, ServerId}
	if t.Status == types.StatusDone {
		http.Redirect(w, r, "/job/"+task+"/output", http.StatusSeeOther)
		//return
	}
	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal(jobStatus)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if t.Status == types.StatusPending {
		w.WriteHeader(http.StatusOK)
	}

	w.Write(js)
}
func deleteTask(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	task, _ := params["id"]
	Quit <- task
	deleteDoneTask(w, r)
}
func deleteDoneTask(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	task, _ := params["id"]
	ctx := context.Background()
	_, err := fsClient.Doc("tasks/" + task).Delete(ctx)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))

}
func getResults(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	task, _ := params["id"]
	ok, t, err := getTask(task)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
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

func getTask(task string) (bool, types.TaskData, error) {
	var t types.TaskData
	ctx := context.Background()
	docsnap, err := fsClient.Doc("tasks/" + task).Get(ctx)
	docsnap.DataTo(&t)
	if err != nil {
		log.Println(err)
		return false, t, errors.New("Task not found: " + task)
	}
	return true, t, nil
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
	rtr := mux.NewRouter()
	rtr.HandleFunc("/job", longTask).Methods(http.MethodPost)
	rtr.HandleFunc("/job/{id:[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}}/{output}", getResults).Methods(http.MethodGet)
	rtr.HandleFunc("/job/{id:[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}}/{output}", deleteDoneTask).Methods(http.MethodDelete)
	rtr.HandleFunc("/job/{id:[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}}", taskStatus).Methods(http.MethodGet)
	rtr.HandleFunc("/job/{id:[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}}", deleteTask).Methods(http.MethodDelete)
	http.Handle("/", rtr)
	err = http.ListenAndServe(hostPort, nil)
	if err != nil {
		log.Fatal(err)
	}
}
