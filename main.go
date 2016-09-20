package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/santrancisco/checkcommit/slackalert"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const (
	//SLACKCHANNEL to alert on
	// SLACKCHANNEL = "#secops-private"
	SLACKCHANNEL = "@santrancisco"
	//POLICE icon
	POLICE = ":oncoming_police_car:"
	//GHOST icon
	GHOST = ":ghost:"
	// ALERT icon
	ALERT = ":rotating_light:"
)

var (
	debugflag     = kingpin.Flag("debug", "Enable debug mode.").Default("false").Short('d').Bool()
	idfileflag    = kingpin.Flag("id", "Save/Get id from file (optional)").Default("false").Bool()
	org           = kingpin.Flag("org", "Github organisation to check").Default("NONE").Short('o').String()
	timer         = kingpin.Flag("timer", "How often in seconds ").Default("60s").Short('t').Duration()
	httpportForCF = kingpin.Flag("port", "create a HTTP listener to satisfy CF healthcheck requirement").Default("1337").OverrideDefaultFromEnvar("HTTPPORT").Short('p').String()
	perpage       = kingpin.Flag("perpage", "configure the number of events return by API").Default("100").OverrideDefaultFromEnvar("PERPAGE").Int()
	slackurl      = os.Getenv("SLACKURL")
	slacktoken    = os.Getenv("SLACKUPLOADTOKEN")
	githubToken   = os.Getenv("GITHUB")
	slackreport   = ""
	// Some regex Note:
	// (?mi) switch is used for multi-line search and case-insensitive
	regexswitch = "(?mi)"
	//we can add for more pattern later
	commitedline = `^\+.*`
	patterns     = []string{`(secret|password|key|token)+(\|\\|\/|\"|')?\s*(:|=>|=)\s*(\|\\|\/|\"|')?[A-Za-z0-9\/\+\\= ]+(\||\\|\/|\"|')?\s*$`}
)

// TODO: PARSING argument using kingpin library

func check(e error) {
	if e != nil {
		//panic(e)
		fmt.Println(e)
		os.Exit(1)
	}
}

func debug(s string) {
	if *debugflag {
		fmt.Println(s)
	}
}

func getIDFromFile() (previousID int) {
	buff, err := ioutil.ReadFile("./id")
	check(err)
	previousID, err = strconv.Atoi(strings.TrimSpace(string(buff)))
	check(err)
	return
}

func saveIDToFile(currentID int) {
	err := ioutil.WriteFile("./id", []byte(strconv.Itoa(currentID)), 0644)
	check(err)
}

func sendtoslack(slackreport string) {
	if slackreport != "" {
		slackreport = "POTENTIAL CREDENTIALS LEAK:\n\n" + slackreport
		notify := slackalert.SlackStruct{URL: slackurl, Uploadtoken: slacktoken, Icon: POLICE, Channel: SLACKCHANNEL}
		notify.UploadFile(time.Now().Format("2006-02-01")+".txt", slackreport)
	}
}

func main() {
	go func() {
		http.ListenAndServe(fmt.Sprintf(":%s", *httpportForCF), new(http.ServeMux))
		panic("health check exited")
	}()

	kingpin.Version("0.0.1")
	kingpin.Parse()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)
	//ListOptions have Page and PerPage options
	opt := &github.ListOptions{PerPage: *perpage}
	previousID := 0
	if *idfileflag {
		debug("[+] id flag is on, acquiring previousid from file!")
		previousID = getIDFromFile()
	}
	for {
		debug("====================================================")
		debug("Polling github API")
		debug(fmt.Sprintf("[+] The previous ID is %d", previousID))
		// Get the latest 100 events from
		events, _, err := client.Activity.ListEventsForOrganization(*org, opt)
		check(err)
		latestID, err := strconv.Atoi(*events[0].ID)
		check(err)
		if *idfileflag {
			saveIDToFile(latestID)
			debug("[+] id flag is on, saving latestID to file!")
		}

		if !(latestID > previousID) {
			debug("[+] No update was made")
		} else {
			for _, event := range events {
				// Check and make sure we are not doubling our work
				currentID, err := strconv.Atoi(*event.ID)
				check(err)
				if currentID > previousID {
					// event.Type is a pointer to string, not a string... apparently?
					// Anyhow, looks for PushEvent here
					if *event.Type == "PushEvent" {
						pushevent := &github.PushEvent{}
						json.Unmarshal(*event.RawPayload, &pushevent)
						debug("[+] Updating: " + *event.Repo.Name)
						for i, commit := range pushevent.Commits {
							thiscommit, _, err := client.Repositories.GetCommit(*org, strings.Split(*event.Repo.Name, "/")[1], *commit.SHA)
							check(err)
							debug("[+] By: " + *commit.Author.Name)
							debug("[+] URL: " + *commit.URL)
							for _, file := range thiscommit.Files {
								debug("[+] " + *file.Status + " " + *file.Filename + ":")
								if file.Patch != nil {
									for _, pattern := range patterns {
										re := regexp.MustCompile(regexswitch + commitedline + pattern)
										matches := re.FindAllString(*file.Patch, -1)
										if len(matches) > 0 {
											slackreport += "====================================================\n"
											slackreport += "[+] Updating: " + *event.Repo.Name + "\n"
											slackreport += "[+] By: " + *commit.Author.Name + "\n"
											slackreport += "[+] URL: " + *thiscommit.HTMLURL + "\n"
											slackreport += "[+] " + *file.Status + " " + *file.Filename + ":" + "\n"
											slackreport += strings.Join(matches, "\n")
											slackreport += "\n\n"
										}
									}
								}
							}
							if i > 1 {
								break
							}
						}
						//fmt.Println("---------------------")
					}
				}
			}
		}
		previousID = latestID
		sendtoslack(slackreport)
		slackreport = ""
		time.Sleep(*timer)
	}
}
