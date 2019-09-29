package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/spf13/viper"
	"github.com/doitintl/long_john_silver/types"
)


func init() {
	viper.SetConfigFile("config.json")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal("ReadInConfig: ", err)
	}
}
var wg sync.WaitGroup
func main() {

	id,err := getlongtask()
	if err != nil {
		log.Println(err)
		return
	}
	for i := 1;  i<=viper.GetInt("requests"); i++{
		wg.Add(1)
		go gettaskstatus(id,i)
	}
	wg.Wait()
	log.Println("We are doen")
}

func getlongtask() (string, error){
	req, _ := http.NewRequest("GET", viper.GetString("url")+"longtask", nil)

	req.Header.Add("Accept", "*/*")
	req.Header.Add("Connection", "keep-alive")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println(err)
		return "",err
	}
	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	var ac AcceptedResponse

	if err = json.Unmarshal(body, &ac); err != nil {
		log.Println(err)
		return "",err
	}

	return ac.Task.Id, nil
}

func gettaskstatus(taskid string, id int) error {
	defer wg.Done()
	for {
		req, _ := http.NewRequest("GET", viper.GetString("url")+"taskstatus?task="+taskid, nil)

		req.Header.Add("Accept", "*/*")
		req.Header.Add("Connection", "keep-alive")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println(err)
			return err
		}
		if res.StatusCode != http.StatusOK{
			return nil
		}
		defer res.Body.Close()
		body, _ := ioutil.ReadAll(res.Body)
		var st StatusResponse
		if err = json.Unmarshal(body, &st); err != nil {
			log.Println(err)
			return err
		}
		log.Println("gettaskstatus: " + strconv.Itoa(id) + " Server :" + st.ServerId + " Task :" + st.Id + " Original Server " + st.TaskData.OriginalServer)
	}
	return nil
}
