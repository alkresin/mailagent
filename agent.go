package main

import (
	"fmt"
	egui "github.com/alkresin/external"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

const NMSGREAD = 5

type embox struct {
	Num     int
	Title   string
	Addr    string
	Login   string
	Pass    string
	Trash   string
	Unseen  int
	Total   int
	LastUid uint32
	bRead   bool
	Amsg    [][]string
	mux     sync.Mutex
}

var sErr = ""
var pBoxes []embox

func main() {

	var sInit string

	{
		ex, _ := os.Executable()
		os.Chdir(filepath.Dir(ex))

		b, err := ioutil.ReadFile("egui.ini")
		if err != nil {
			sInit = ""
		} else {
			sInit = string(b)
		}
	}
	if egui.Init(sInit) != 0 {
		return
	}

	defer egui.Exit()

	egui.RegFunc("setbox", setBox)
	egui.RegFunc("getinfo", getInfo)
	egui.RegFunc("getresult", getResult)
	egui.RegFunc("getmsgs", getMsgs)
	egui.RegFunc("delmsgs", delMsgs)

	pBoxes = make([]embox, 10)

	egui.OpenMainForm("data/main.xml")

}

func getLastMessages(i int, c *client.Client, mbox *imap.MailboxStatus) {

	var sFrom string

	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > (NMSGREAD - 1) {
		from = mbox.Messages - (NMSGREAD - 1)
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid}, messages)
	}()

	if pBoxes[i].Amsg == nil {
		pBoxes[i].Amsg = make([][]string, NMSGREAD)
	} else {
		for j := 0; j < NMSGREAD; j++ {
			pBoxes[i].Amsg[j] = nil
		}
	}
	j := NMSGREAD - 1
	for msg := range messages {
		sFrom = msg.Envelope.Sender[0].PersonalName
		if sFrom == "" {
			sFrom = msg.Envelope.Sender[0].MailboxName + "@" + msg.Envelope.Sender[0].HostName
		}
		sUnSeen := "true"
		for _, flag := range msg.Flags {
			if flag == imap.SeenFlag {
				sUnSeen = "false"
				break
			}
		}
		pBoxes[i].Amsg[j] = []string{sFrom, msg.Envelope.Subject, fmt.Sprintf("%s", msg.Envelope.Date)[:19], sUnSeen, fmt.Sprintf("%d", msg.Uid)}
		j--
	}

	if err := <-done; err != nil {
		sErr = fmt.Sprintln(err)
		return
	}
}

func getMessages(i int, c *client.Client) {

	var uiLastUid uint32 = 0
	var err error

	sErr = ""

	if c == nil {
		c, err = client.DialTLS(pBoxes[i].Addr, nil)
		if err != nil {
			sErr = fmt.Sprintln(err)
			return
		}

		defer c.Logout()

		// Login
		if err = c.Login(pBoxes[i].Login, pBoxes[i].Pass); err != nil {
			sErr = fmt.Sprintln(err)
			return
		}
	}

	// Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		sErr = fmt.Sprintln(err)
		return
	}

	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{"\\Seen"}
	uids, err := c.Search(criteria)
	if err != nil {
		sErr = fmt.Sprintln(err)
		return
	}
	if len(uids) > 0 {
		uiLastUid = uids[len(uids)-1]
	}

	if pBoxes[i].Unseen != len(uids) || pBoxes[i].Total != int(mbox.Messages) ||
		pBoxes[i].LastUid != uiLastUid {
		pBoxes[i].mux.Lock()
		pBoxes[i].Unseen = len(uids)
		pBoxes[i].Total = int(mbox.Messages)
		pBoxes[i].LastUid = uiLastUid
		pBoxes[i].mux.Unlock()

		// Get the last 5 messages
		getLastMessages(i, c, mbox)
	}

}

func setBox(p []string) string {

	i, _ := strconv.Atoi(p[0])
	i--
	pBoxes[i] = embox{Title: p[1], Addr: p[2], Login: p[3], Pass: p[4], Trash: p[5], Unseen: -1, Total: -1}

	return ""
}

func checkMessages(i int) {

	pBoxes[i].mux.Lock()
	pBoxes[i].bRead = true
	pBoxes[i].mux.Unlock()

	getMessages(i, nil)

	pBoxes[i].mux.Lock()
	pBoxes[i].bRead = false
	if sErr != "" {
		pBoxes[i].Unseen = -2
		pBoxes[i].Total = -2
	}
	pBoxes[i].mux.Unlock()

	if sErr != "" {
		egui.WriteLog(sErr)
	}
}

func delMessages(i int, p []string) {

	var ui uint64

	if len(p) == 0 {
		return
	}

	c, err := client.DialTLS(pBoxes[i].Addr, nil)
	if err != nil {
		sErr = fmt.Sprintln(err)
		return
	}

	defer c.Logout()

	// Login
	if err := c.Login(pBoxes[i].Login, pBoxes[i].Pass); err != nil {
		sErr = fmt.Sprintln(err)
		return
	}

	// Select INBOX
	_, err = c.Select("INBOX", false)
	if err != nil {
		sErr = fmt.Sprintln(err)
		return
	}

	//fmt.Println(pBoxes[i].Title)
	seqset := new(imap.SeqSet)
	for _, s := range p {
		ui, _ = strconv.ParseUint(s, 10, 32)
		seqset.AddNum(uint32(ui))
		//fmt.Println(ui)
	}

	criteria := imap.NewSearchCriteria()
	criteria.Uid = seqset
	uids, err := c.Search(criteria)
	if err != nil {
		sErr = fmt.Sprintln(err)
		return
	}
	seqset.Clear()
	if len(uids) > 0 {
		seqset.AddNum(uids...)
	}

	if !seqset.Empty() {
		if pBoxes[i].Trash != "" {
			if err := c.Copy(seqset, pBoxes[i].Trash); err != nil {
				egui.WriteLog( fmt.Sprintln(err) )
			}
		}
		item := imap.FormatFlagsOp(imap.AddFlags, true)
		flags := []interface{}{imap.DeletedFlag}
		if err := c.Store(seqset, item, flags, nil); err != nil {
			sErr = fmt.Sprintln(err)
			return
		}

		if err := c.Expunge(nil); err != nil {
			sErr = fmt.Sprintln(err)
			return
		}
		getMessages(i, c)
	}
}

func getInfo(p []string) string {

	i, _ := strconv.Atoi(p[0])
	i--
	if pBoxes[i].Addr != "" {
		go checkMessages(i)
	}

	return ""
}

func getResult(p []string) string {

	var Unseen, Total int
	var LastUid uint32

	i, _ := strconv.Atoi(p[0])
	i--
	pBoxes[i].mux.Lock()
	if pBoxes[i].bRead {
		Unseen = -1
		Total = -1
		LastUid = 0
	} else {
		Unseen = pBoxes[i].Unseen
		Total = pBoxes[i].Total
		LastUid = pBoxes[i].LastUid
	}
	pBoxes[i].mux.Unlock()

	return fmt.Sprintf("[%d,%d,%d]", Unseen, Total, LastUid)
}

func getMsgs(p []string) string {

	i, _ := strconv.Atoi(p[0])
	i--
	return fmt.Sprintf("%s", egui.ToString(pBoxes[i].Amsg))
}

func delMsgs(p []string) string {

	i, _ := strconv.Atoi(p[0])
	i--

	pBoxes[i].mux.Lock()
	pBoxes[i].bRead = true
	pBoxes[i].mux.Unlock()

	delMessages(i, p[1:])

	pBoxes[i].mux.Lock()
	pBoxes[i].bRead = false
	if sErr != "" {
		pBoxes[i].Unseen = -2
		pBoxes[i].Total = -2
	}
	pBoxes[i].mux.Unlock()

	if sErr != "" {
		egui.WriteLog(sErr)
	}
	return ""
}
