package main

type challengeSpec struct {
	ChallengeCode string
	Description   string
	Title         string
	Stage         int
}

var challenges = map[string]challengeSpec{
	"BASIC": challengeSpec{
		ChallengeCode: "BASIC",
		Description:   "Use curl to fetch a page on go.fast",
		Title:         "Basic GET",
		Stage:         0,
	},
	"BASICAUTH": challengeSpec{
		ChallengeCode: "BASICAUTH",
		Description:   "Use curl to fetch a page on go.fast but using a HTTP user name and password, doesnt matter what one you use",
		Title:         "Authorized basic GET",
		Stage:         1,
	},
	"HEADER": challengeSpec{
		ChallengeCode: "HEADER",
		Description:   "Use curl to fetch a page on go.fast but send a header of 'X-Hello: World' with your request",
		Title:         "GET with a header",
		Stage:         1,
	},
	"DELETE": challengeSpec{
		ChallengeCode: "DELETE",
		Description:   "Fire off a DELETE request using to go.fast using curl",
		Title:         "DELETE Request",
		Stage:         1,
	},
	"UPLOAD": challengeSpec{
		ChallengeCode: "UPLOAD",
		Description:   "Upload the contents of /etc/passwd to go.fast in a raw POST",
		Title:         "Custom POST",
		Stage:         2,
	},
	"UPLOADBINARY": challengeSpec{
		ChallengeCode: "UPLOADBINARY",
		Description:   "Upload /usr/bin/curl to go.fast in a raw POST using curl",
		Title:         "Upload a file",
		Stage:         2,
	},
	"UPLOADMIME": challengeSpec{
		ChallengeCode: "UPLOADMIME",
		Description:   "Use curl send a mulipart upload containing a=b somewhere to go.fast",
		Title:         "Multipart Upload",
		Stage:         2,
	},
	"UPLOADVALUES": challengeSpec{
		ChallengeCode: "UPLOADVALUES",
		Description:   "Use curl send a urlencoded POST with a=b to go.fast",
		Title:         "Urlencoded POST Upload",
		Stage:         2,
	},
	"UPLOADMIMEFILE": challengeSpec{
		ChallengeCode: "UPLOADMIMEFILE",
		Description:   "Use curl send a mulipart upload containing both a=b and /usr/bin/curl to go.fast",
		Title:         "Multipart File Upload",
		Stage:         3,
	},
	"FTPGET": challengeSpec{
		ChallengeCode: "FTPGET",
		Description:   "download hello.txt from the FTP server on go.fast",
		Title:         "Download file from FTP server",
		Stage:         4,
	},
	"FTPUPLOAD": challengeSpec{
		ChallengeCode: "FTPUPLOAD",
		Description:   "Upload /usr/bin/curl to the FTP server on go.fast",
		Title:         "Upload file to FTP server",
		Stage:         4,
	},
	"SMTP": challengeSpec{
		ChallengeCode: "SMTP",
		Description:   "Upload /usr/bin/curl to the FTP server on go.fast",
		Title:         "Send a email to blog@benjojo.co.uk to the go.fast mail server",
		Stage:         5,
	},
}
