package types

type JobStatus string

const (
	StatusPending JobStatus = "PENDING"
	StatusDone    JobStatus = "Done"
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
	TaskData TaskData `json:"taskdata"`
	Id       string   `json:"id"`
	ServerId string   `json:"serverid"`
}

type TaskData struct {
	Result string `json:"result"`
	Status JobStatus `json:"jobstatus"`
	Duration string `json:"duration"`
	OriginalServer string `json:"originalserver"`

}

