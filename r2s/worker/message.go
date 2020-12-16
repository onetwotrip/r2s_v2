package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const footerMsg = `
Показаны %d нод(ы) из %d, остальные ищи в логах.
Ошибки могут быть из-за того, что не забутстраплены беты/сенды из списка, 
на этих нодах высокое значение Load Average либо остановлен redis@shared.service.
Для более подробной информации включи DEBUG в джобе.
`

type slackMessage struct {
	Attachments []attachmentStruct `json:"attachments"`
}

type attachmentStruct struct {
	Color     string         `json:"color"`
	Title     string         `json:"title"`
	TitleLink string         `json:"title_link"`
	Text      string         `json:"text"`
	Fields    []fieldsStruct `json:"fields"`
	Footer    string         `json:"footer"`
}

type fieldsStruct struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

func (w *workerStruct) appendErrorHost(name string) {
	w.mt.Lock()
	w.errorHosts = append(w.errorHosts, fieldsStruct{
		Title: "Node",
		Value: name,
		Short: true,
	})
	w.mt.Unlock()
}

func (w *workerStruct) sendSlackMessage() error {
	var lastFourNode []fieldsStruct
	if len(w.errorHosts) > 4 {
		lastFourNode = w.errorHosts[0:4]
	} else {
		lastFourNode = w.errorHosts
	}
	msg := &slackMessage{
		Attachments: []attachmentStruct{
			{
				Color:     "warning",
				Title:     "Ошибка копирования справочников",
				TitleLink: w.config.BuildUrl,
				Text:      "На следующие ноды не удалось скопировать справочники (refs:*)\nиз продового редиса:",
				Fields:    lastFourNode,
				Footer:    fmt.Sprintf(footerMsg, len(lastFourNode), len(w.errorHosts)),
			},
		},
	}
	msgBody, err := json.Marshal(&msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, w.config.SlackHookUrl, bytes.NewBuffer(msgBody))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	if buf.String() != "ok" {
		return errors.New("response from slack not ok")
	}
	return nil
}
