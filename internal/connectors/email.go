package connectors

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net"
	"net/smtp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	_ "github.com/emersion/go-message/charset" // decode non-UTF-8 messages
	"github.com/emersion/go-message/mail"

	"my-agent/internal/config"
	"my-agent/internal/i18n"
)

// Email is a message read over IMAP.
type Email struct {
	UID     uint32
	Date    time.Time
	From    string
	To      string
	Subject string
	Seen    bool
	Body    string
}

// Header is a one-line summary of the message.
func (e Email) Header() string {
	flag := " "
	if !e.Seen {
		flag = "*"
	}
	return fmt.Sprintf("%s uid=%d  %s  %s  %s", flag, e.UID, e.Date.Format("2006-01-02 15:04"), e.From, e.Subject)
}

func dialIMAP(ctx context.Context, e config.EmailConn) (*client.Client, error) {
	if e.IMAPHost == "" {
		return nil, i18n.Err("connectors.err.no_imap")
	}
	port := e.IMAPPort
	if port == 0 {
		port = 993
	}
	addr := net.JoinHostPort(e.IMAPHost, strconv.Itoa(port))
	var c *client.Client
	var err error
	if port == 143 {
		if c, err = client.Dial(addr); err == nil {
			err = c.StartTLS(&tls.Config{ServerName: e.IMAPHost})
		}
	} else {
		c, err = client.DialTLS(addr, &tls.Config{ServerName: e.IMAPHost})
	}
	if err != nil {
		return nil, err
	}
	c.Timeout = 60 * time.Second
	if dl, ok := ctx.Deadline(); ok {
		c.Timeout = time.Until(dl)
	}
	if err := c.Login(e.Username, e.ResolvePassword()); err != nil {
		c.Logout()
		return nil, err
	}
	return c, nil
}

func folder(e config.EmailConn, f string) string {
	if f != "" {
		return f
	}
	if e.Folder != "" {
		return e.Folder
	}
	return "INBOX"
}

// SearchEmail lists messages, newest first. query searches the text of the
// messages (empty: every message); unseen keeps only unread ones.
func SearchEmail(ctx context.Context, e config.EmailConn, box, query string, unseen bool, limit int, withBody bool) ([]Email, error) {
	c, err := dialIMAP(ctx, e)
	if err != nil {
		return nil, err
	}
	defer c.Logout()
	if _, err := c.Select(folder(e, box), true); err != nil {
		return nil, err
	}
	crit := imap.NewSearchCriteria()
	if query != "" {
		crit.Text = []string{query}
	}
	if unseen {
		crit.WithoutFlags = []string{imap.SeenFlag}
	}
	uids, err := c.UidSearch(crit)
	if err != nil {
		return nil, err
	}
	sort.Slice(uids, func(i, j int) bool { return uids[i] > uids[j] })
	if limit <= 0 {
		limit = 20
	}
	if len(uids) > limit {
		uids = uids[:limit]
	}
	if len(uids) == 0 {
		return nil, nil
	}
	out, err := fetch(c, uids, withBody)
	sort.Slice(out, func(i, j int) bool { return out[i].UID > out[j].UID })
	return out, err
}

// ReadEmail returns one message with its text body.
func ReadEmail(ctx context.Context, e config.EmailConn, box string, uid uint32) (Email, error) {
	c, err := dialIMAP(ctx, e)
	if err != nil {
		return Email{}, err
	}
	defer c.Logout()
	if _, err := c.Select(folder(e, box), true); err != nil {
		return Email{}, err
	}
	msgs, err := fetch(c, []uint32{uid}, true)
	if err != nil {
		return Email{}, err
	}
	if len(msgs) == 0 {
		return Email{}, i18n.Errorf("connectors.err.no_message", uid)
	}
	return msgs[0], nil
}

func fetch(c *client.Client, uids []uint32, withBody bool) ([]Email, error) {
	set := new(imap.SeqSet)
	set.AddNum(uids...)
	section := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchUid, imap.FetchFlags}
	if withBody {
		items = append(items, section.FetchItem())
	}
	ch := make(chan *imap.Message, 16)
	done := make(chan error, 1)
	go func() { done <- c.UidFetch(set, items, ch) }()
	var out []Email
	for m := range ch {
		em := Email{UID: m.Uid}
		if env := m.Envelope; env != nil {
			em.Date, em.Subject = env.Date, env.Subject
			em.From, em.To = addrs(env.From), addrs(env.To)
		}
		for _, f := range m.Flags {
			if f == imap.SeenFlag {
				em.Seen = true
			}
		}
		if withBody {
			if r := m.GetBody(section); r != nil {
				em.Body = bodyText(r)
			}
		}
		out = append(out, em)
	}
	return out, <-done
}

func addrs(list []*imap.Address) string {
	var parts []string
	for _, a := range list {
		addr := a.Address()
		if a.PersonalName != "" {
			addr = fmt.Sprintf("%s <%s>", a.PersonalName, addr)
		}
		parts = append(parts, addr)
	}
	return strings.Join(parts, ", ")
}

// bodyText extracts the plain text of a message, falling back to its HTML.
func bodyText(r io.Reader) string {
	mr, err := mail.CreateReader(r)
	if err != nil {
		b, _ := io.ReadAll(io.LimitReader(r, maxBytes))
		return string(b)
	}
	var plain, html string
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		h, ok := p.Header.(*mail.InlineHeader)
		if !ok {
			continue
		}
		ct, _, _ := h.ContentType()
		b, _ := io.ReadAll(io.LimitReader(p.Body, maxBytes))
		switch {
		case ct == "text/plain" && plain == "":
			plain = string(b)
		case ct == "text/html" && html == "":
			html = HTMLToText(string(b))
		}
	}
	if strings.TrimSpace(plain) != "" {
		return strings.TrimSpace(plain)
	}
	return html
}

// SendEmail sends a plain-text message over SMTP (STARTTLS on 587/25,
// implicit TLS on 465).
func SendEmail(ctx context.Context, e config.EmailConn, to, cc []string, subject, body string) error {
	if e.SMTPHost == "" {
		return i18n.Err("connectors.err.no_smtp")
	}
	if len(to) == 0 {
		return i18n.Err("connectors.err.no_recipient")
	}
	port := e.SMTPPort
	if port == 0 {
		port = 587
	}
	from := e.From
	if from == "" {
		from = e.Username
	}
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	var msg strings.Builder
	fmt.Fprintf(&msg, "From: %s\r\nTo: %s\r\n", from, strings.Join(to, ", "))
	if len(cc) > 0 {
		fmt.Fprintf(&msg, "Cc: %s\r\n", strings.Join(cc, ", "))
	}
	fmt.Fprintf(&msg, "Subject: %s\r\nDate: %s\r\nMessage-ID: <%s@my-agent>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n", // i18n:ok (MIME headers)
		mime.QEncoding.Encode("utf-8", subject), time.Now().Format(time.RFC1123Z), hex.EncodeToString(id))
	msg.WriteString(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))

	addr := net.JoinHostPort(e.SMTPHost, strconv.Itoa(port))
	auth := smtp.PlainAuth("", e.Username, e.ResolvePassword(), e.SMTPHost)
	rcpt := append(append([]string{}, to...), cc...)
	if port != 465 {
		return smtp.SendMail(addr, auth, from, rcpt, []byte(msg.String()))
	}
	d := &net.Dialer{Timeout: 30 * time.Second}
	conn, err := tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: e.SMTPHost})
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, e.SMTPHost)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Auth(auth); err != nil {
		return err
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, r := range rcpt {
		if err := c.Rcpt(r); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg.String())); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// EmailLoader ingests the latest messages of the folder.
type EmailLoader struct {
	Name string
	Conn config.EmailConn
}

func (l EmailLoader) Load(ctx context.Context) ([]Document, error) {
	limit := l.Conn.Limit
	if limit <= 0 {
		limit = 50
	}
	msgs, err := SearchEmail(ctx, l.Conn, "", "", false, limit, true)
	if err != nil {
		return nil, err
	}
	docs := make([]Document, 0, len(msgs))
	for _, m := range msgs {
		text := fmt.Sprintf("From: %s\nTo: %s\nDate: %s\nSubject: %s\n\n%s", m.From, m.To, m.Date.Format(time.RFC1123), m.Subject, m.Body)
		docs = append(docs, Document{Source: fmt.Sprintf("email:%s/%s/%d", l.Name, folder(l.Conn, ""), m.UID), Text: text})
	}
	return docs, nil
}
