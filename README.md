### What is this?

This golang app will monitor api.github.com/orgs/ORG/events every 60 seconds and look for any commits that may leak the credentials but can be used to look for any other patterns.

Users can add more pattern to search for in patterns array in the code. The current patterns array has 1 pattern:
```
patterns     = []string{`(secret|password|key|token)+(\|\\|\/|\"|')?\s*(:|=>|=)\s*(\|\\|\/|\"|')?[A-Za-z0-9\/\+\\= ]+(\||\\|\/|\"|')?\s*$`}
```

Any patterns used for searching will be prepended with  "(?mi)^\+.*" and that is to enabled :

 - multi line search
 - case insensitive search
 - any line starting with "+" to make sure we only looks for stuffs that was modified/added.

The app will have HTTP listener on port 1337 that return a 404 because CF health monitoring needs this...
```
Usage: cron_job [<flags>]

Flags:
      --help          Show context-sensitive help (also try --help-long and --help-man).
  -d, --debug         Enable debug mode.
      --id            Save/Get id from file
  -o, --org="AusDTO"  Github organisation to check
  -t, --timer=60s     How often in seconds
  -p, --port="1337"   create a HTTP listener to satisfy CF healthcheck requirement
      --version       Show application version.

```

Before pushing app to cloudfoundry, you probably want to package all dependencies into vendor folder using godep:

```
godep save
cf push
```
