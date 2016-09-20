package slackalert

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	//"reflect"
)

// Stole this code from here: https://astaxie.gitbooks.io/build-web-application-with-golang/content/en/04.5.html
func postFile(filename string, content string, targetURL string) error {
	bodyBuf := &bytes.Buffer{}
	bodyWriter := multipart.NewWriter(bodyBuf)

	// this step is very important
	fileWriter, err := bodyWriter.CreateFormFile("file", filename)
	if err != nil {
		fmt.Println("error writing to buffer")
		return err
	}
	fileWriter.Write([]byte(content))
	contentType := bodyWriter.FormDataContentType()
	bodyWriter.Close()

	resp, err := http.Post(targetURL, contentType, bodyBuf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respbody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Println(resp.Status)
	fmt.Println(string(respbody))
	return nil
}

//UploadFile sends text file to channel
func (s *SlackStruct) UploadFile(filename string, content string) error {
	var slackurl *url.URL
	slackurl, _ = url.Parse("https://slack.com")
	slackurl.Path += "/api/files.upload"
	params := url.Values{}
	params.Add("token", s.Uploadtoken)
	params.Add("channels", s.Channel)
	params.Add("filetype", "text")
	params.Add("pretty", "1")
	slackurl.RawQuery = params.Encode()
	fmt.Println(slackurl.String())
	return postFile(filename, content, slackurl.String())
}
