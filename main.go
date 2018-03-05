package main

import (
	"encoding/json"
	"fmt"
	"io"
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
	//POLICE icon
	POLICE = ":oncoming_police_car:"
	//GHOST icon
	GHOST = ":ghost:"
	// ALERT icon
	ALERT = ":rotating_light:"
)

var (
	slackchannel  = kingpin.Flag("slack", "Set the name of slack channel this alert goes to").Default("@santrancisco").OverrideDefaultFromEnvar("CHECK_SLACK").Short('s').String()
	debugflag     = kingpin.Flag("debug", "Enable debug mode.").Default("false").Short('d').Bool()
	idfileflag    = kingpin.Flag("id", "Save/Get id from file (optional)").Default("false").Bool()
	org           = kingpin.Flag("org", "Github organisation to check").Default("NONE").OverrideDefaultFromEnvar("CHECK_ORG").Short('o').String()
	timer         = kingpin.Flag("timer", "How often in seconds ").Default("60s").Short('t').OverrideDefaultFromEnvar("CHECK_TIMER").Duration()
	httpportForCF = kingpin.Flag("port", "create a HTTP listener to satisfy CF healthcheck requirement").Default("8080").OverrideDefaultFromEnvar("VCAP_APP_PORT").Short('p').String()
	perpage       = kingpin.Flag("perpage", "configure the number of events return by API").Default("100").OverrideDefaultFromEnvar("CHECK_PERPAGE").Int()
	slackurl      = os.Getenv("CHECK_SLACKURL")
	slacktoken    = os.Getenv("CHECK_SLACKUPLOADTOKEN")
	githubToken   = os.Getenv("CHECK_GITHUB")
	githubclient  *github.Client
	slackreport   = ""
	// Some regex Note:
	// (?mi) switch is used for multi-line search and case-insensitive
	regexswitch = "(?mi)"
	//we can add for more pattern later
	commitedlineregex = `(?mi)^\+.*`
	// matching anything that have secret,password,key,token at the end of the variable and have assignment directive (:|=>|=)
	patterns = []string{`(?mi)(secret|password|key|token)+(\|\\|\/|\"|')?\s*(:|=>|=)\s*.*?(\)|\"|'|\s|$)`}
	//falsepositive list - matching anything that has "env"
	falsepositive   = []string{`(?mi)^.*(=|=>|:).*(env|fake).*`, 
	                           `(?mi)^.*(=|=>|:)(\)|\"|'|\s)*(true|false)(\)|\"|'|\s|$)`, 
														 `(?mi)^.*(=|=>|:)(\)|\"|'|\s)*(\$.*)(\)|\"|'|\s|$)`,
													   `(/?mi)^.*(=|=>|:)(\)|\"|'|\s)*\(\(.*\)\)(\)|\"|'|\s)`,
														 `(?mi)^.*(=|=>|:)(\)|\"|'|\s)*\{\{.*\}\}(\)|\"|'|\s)`,
												 }
	ignoreextension = []string{"html", "js", "css"}
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

//Send slackreport to slack
func sendtoslack(slackreport string) {
	debug(slackreport)
	if (slackreport != "") && (!*debugflag) {
		slackreport = "POTENTIAL CREDENTIALS LEAK:\n\n" + slackreport
		notify := slackalert.SlackStruct{URL: slackurl, Uploadtoken: slacktoken, Icon: POLICE, Channel: *slackchannel}
		notify.Sendmsg("Incoming falsch positiv aufmerksam!")
		notify.UploadFile(time.Now().Format("2006-02-01")+".txt", slackreport)
	}
}

//HelloServer some stuffs
func HelloServer(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "I am alive!\n")
}

//Processing list of events
func processEvents(events []github.Event, previousID int) (interestingEvents []github.Event) {
	for _, event := range events {
		// Check and make sure we are not doubling our work
		currentID, err := strconv.Atoi(*event.ID)
		check(err)
		// event.Type is a pointer to string, not a string... apparently?
		// Anyhow, looks for PushEvent here
		if (currentID > previousID) && (*event.Type == "PushEvent") {
			interestingEvents = append(interestingEvents, event)
			//fmt.Println("---------------------")
		}
	}
	return
}

// Check if this is a false positive result
func isFalsePositive(match string) (falsepositiveresult bool) {
	falsepositiveresult = false
	for _, pattern := range falsepositive {
		re := regexp.MustCompile(pattern)
		//fmt.Println(match)
		if re.MatchString(match) {
			falsepositiveresult = true
			break
		}
	}
	return
}

// Search for interesting patterns
func searchPattern(commitlines []string) (matches []string) {
	for _, match := range commitlines {
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			//fmt.Println(match)
			if re.MatchString(match) {
				narrowscope := re.FindAllString(match, -1)[0]
				if !isFalsePositive(narrowscope) {
					matches = append(matches, match)
					break
				}
			}

		}
	}
	return
}

//Process each modified files and look for interesting patterns
func processFilePatch(file github.CommitFile) (report string) {
	//Check if file is in the list of ignoreextension
	debug("Process file")
	report = ""
	filename := strings.Split(*file.Filename, ".")
	ignorefile := false
	for _, i := range ignoreextension {
		if filename[len(filename)-1] == i {
			ignorefile = true
		}
	}
	//Return if file is in the ignore list or there is no patch
	if ignorefile || (file.Patch == nil) {
		debug(*file.Filename + " was ignored")
		return
	}
	debug("[+] " + *file.Status + " " + *file.Filename + ":")
	re := regexp.MustCompile(commitedlineregex)
	commitedlines := re.FindAllString(*file.Patch, -1)
	matches := searchPattern(commitedlines)
	if len(matches) > 0 {
		report += "[+] " + *file.Status + " " + *file.Filename + ":" + "\n"
		report += strings.Join(matches, "\n")
		report += "\n\n"
	}
	return
}

//Proccess each push event
func processPushEvent(pushevent *github.PushEvent, reponame string) (report string) {
	report = ""
	for _, commit := range pushevent.Commits {
		thiscommit, _, err := githubclient.Repositories.GetCommit(*org, strings.Split(reponame, "/")[1], *commit.SHA)
		check(err)
		debug("[+] By: " + *commit.Author.Name)
		debug("[+] URL: " + *commit.URL)
		for _, file := range thiscommit.Files {
			filereport := processFilePatch(file)
			if filereport == "" {
				continue
			}
			filereport = "[+] URL: " + *thiscommit.HTMLURL + "\n" + filereport
			filereport = "[+] By: " + *commit.Author.Name + "\n" + filereport
			filereport = "[+] Updating: " + reponame + "\n" + filereport
			report += filereport
		}
	}
	return
}

func main() {
	kingpin.Version("0.0.1")
	kingpin.Parse()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
	)

	go func() {
		http.HandleFunc("/", HelloServer)
		http.ListenAndServe(fmt.Sprintf(":%s", *httpportForCF), nil)
		panic("health check exited")
	}()

	tc := oauth2.NewClient(oauth2.NoContext, ts)
	githubclient = github.NewClient(tc)
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
		events, _, err := githubclient.Activity.ListEventsForOrganization(*org, opt)
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
			interestingEvents := processEvents(events, previousID)
			for _, event := range interestingEvents {
				pushevent := &github.PushEvent{}
				json.Unmarshal(*event.RawPayload, &pushevent)
				debug("[+] Updating: " + *event.Repo.Name)
				pusheventreport := processPushEvent(pushevent, *event.Repo.Name)
				if pusheventreport == "" {
					continue
				}
				pusheventreport = "[+] Event id: " + *event.ID + "\n" + pusheventreport
				pusheventreport = "====================================================\n" + pusheventreport
				slackreport += pusheventreport
			}
		}

		previousID = latestID
		sendtoslack(slackreport)
		slackreport = ""
		time.Sleep(*timer)
	}
}
