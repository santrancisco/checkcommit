package slackalert

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

//MessageStruct struct for parsing json. we are interested in name and language
type messageStruct struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
	Icon    string `json:"icon_emoji"`
}

//SlackStruct class Uploadtoken can be acquired here per user https://api.slack.com/methods/files.upload/test
//URL is the webhook URL.
type SlackStruct struct {
	URL         string
	Uploadtoken string
	Icon        string
	Channel     string
}

//Sendmsg sends messages method
func (s *SlackStruct) Sendmsg(msg string) error {
	slackobject := &messageStruct{Channel: s.Channel, Text: msg, Icon: s.Icon}
	jsonbytes, err := json.Marshal(slackobject)
	data := url.Values{}
	data.Add("payload", string(jsonbytes))
	_, err = http.PostForm(s.URL, data)
	if err != nil {
		return err
	}
	return nil
}

//SendRawSlack send raw slack message with all input in the argument
func SendRawSlack(targeturl string, channel string, msg string, icon string) error {
	slackobject := &messageStruct{Channel: channel, Text: msg, Icon: icon}
	jsonbytes, err := json.Marshal(slackobject)
	data := url.Values{}
	data.Add("payload", string(jsonbytes))
	fmt.Print(slackobject.Text)
	_, err = http.PostForm(targeturl, data)
	if err != nil {
		return err
	}
	return nil
}
