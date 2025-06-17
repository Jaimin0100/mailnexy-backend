// utils/verifier.go
package utils

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/badoux/checkmail"
	"github.com/likexian/whois"
)

type VerificationResult struct {
	Email        string `json:"email"`
	Status       string `json:"status"` // valid, invalid, disposable, catch-all, unknown
	Details      string `json:"details"`
	IsReachable  bool   `json:"is_reachable"`
	IsBounceRisk bool   `json:"is_bounce_risk"`
	WHOIS        string `json:"whois,omitempty"`
}

var (
	// Expanded disposable domains (500+ domains)
	disposableDomains = loadDisposableDomains()

	// Major free email providers
	freeEmailProviders = []string{
		"gmail.com", "yahoo.com", "outlook.com", "hotmail.com",
		"aol.com", "protonmail.com", "icloud.com", "mail.com",
		"yandex.com", "zoho.com", "gmx.com",
	}

	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// Common email typos
	commonTypos = map[string]string{
		"gmai.com":   "gmail.com",
		"gmal.com":   "gmail.com",
		"gmail.co":   "gmail.com",
		"yaho.com":   "yahoo.com",
		"hotmai.com": "hotmail.com",
		"outlok.com": "outlook.com",
	}

	// Domain to MX cache
	mxCache = struct {
		sync.RWMutex
		m map[string][]*net.MX
	}{m: make(map[string][]*net.MX)}

	// SMTP connection timeout
	smtpTimeout = 15 * time.Second
)

// EnhancedVerifyEmailAddress performs comprehensive email verification
func EnhancedVerifyEmailAddress(email string) (*VerificationResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	result := &VerificationResult{
		Email:        email,
		Status:       "unknown",
		IsReachable:  false,
		IsBounceRisk: true,
	}

	// 1. Basic syntax validation using checkmail
	if err := checkmail.ValidateFormat(email); err != nil {
		result.Status = "invalid"
		result.Details = "Invalid email format: " + err.Error()
		return result, nil
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		result.Status = "invalid"
		result.Details = "Invalid email format"
		return result, nil
	}

	localPart, domain := parts[0], parts[1]

	// 2. Check for common typos
	if suggestedDomain, ok := commonTypos[domain]; ok {
		result.Status = "invalid"
		result.Details = fmt.Sprintf("Possible typo, did you mean %s@%s?", localPart, suggestedDomain)
		return result, nil
	}

	// 3. Disposable email check
	if isDisposableDomain(domain) {
		result.Status = "disposable"
		result.Details = "Disposable email domain"
		return result, nil
	}

	// 4. DNS/MX record check with checkmail
	if err := checkmail.ValidateHost(domain); err != nil {
		result.Status = "invalid"
		result.Details = "Domain validation failed: " + err.Error()
		return result, nil
	}

	// 5. Enhanced SMTP verification
	smtpResult, err := enhancedVerifySMTP(domain, email)
	if err != nil {
		return result, err
	}

	// 6. Add WHOIS data if available
	if whoisInfo, err := whois.Whois(domain); err == nil {
		smtpResult.WHOIS = whoisInfo
	}

	return smtpResult, nil
}

// ExtractDomain extracts domain from email address
func ExtractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func isDisposableDomain(domain string) bool {
	return disposableDomains[domain]
}

func isFreeEmailProvider(domain string) bool {
	for _, provider := range freeEmailProviders {
		if domain == provider {
			return true
		}
	}
	return false
}

func enhancedVerifySMTP(domain, email string) (*VerificationResult, error) {
	result := &VerificationResult{
		Email:        email,
		Status:       "unknown",
		IsReachable:  false,
		IsBounceRisk: true,
	}

	// Try checkmail's verification first
	if _, err := net.LookupMX(domain); err != nil {
		result.Details = "Domain verification failed: " + err.Error()
		return result, nil
	}

	// Get MX records with caching
	mxRecords, err := getMXRecords(domain)
	if err != nil || len(mxRecords) == 0 {
		result.Status = "invalid"
		result.Details = "Domain has no MX records"
		return result, nil
	}

	// Try multiple MX servers
	for _, mx := range mxRecords {
		mailServer := strings.TrimSuffix(mx.Host, ".")

		// Try common ports
		portsToTry := []string{"25", "587", "465"}
		if isFreeEmailProvider(domain) {
			// For free providers, try submission ports first
			portsToTry = []string{"587", "465", "25"}
		}

		for _, port := range portsToTry {
			addr := fmt.Sprintf("%s:%s", mailServer, port)
			smtpResult, err := checkSMTP(addr, domain, email)
			if err == nil {
				return smtpResult, nil
			}
		}
	}

	// Fallback to checkmail's mailbox verification
	// verifier := checkmail.NewVerifier()
	// verifier.FromEmail = "noreply@mailnexy.com"
	// if err := verifier.Verify(email); err == nil {
	// 	result.Status = "valid"
	// 	result.Details = "Mailbox verified"
	// 	result.IsReachable = true
	// 	result.IsBounceRisk = false
	// 	return result, nil
	// }

	result.Details = "All verification attempts failed"
	return result, nil
}

func getMXRecords(domain string) ([]*net.MX, error) {
	// Check cache first
	mxCache.RLock()
	if records, ok := mxCache.m[domain]; ok {
		mxCache.RUnlock()
		return records, nil
	}
	mxCache.RUnlock()

	// Lookup fresh records with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var resolver net.Resolver
	mxRecords, err := resolver.LookupMX(ctx, domain)
	if err != nil {
		return nil, err
	}

	// Update cache
	mxCache.Lock()
	mxCache.m[domain] = mxRecords
	mxCache.Unlock()

	return mxRecords, nil
}

func checkSMTP(addr, domain, email string) (*VerificationResult, error) {
	conn, err := net.DialTimeout("tcp", addr, smtpTimeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, domain)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	// Set timeout for each SMTP command
	deadline := time.Now().Add(smtpTimeout)
	conn.SetDeadline(deadline)

	// 1. Send HELO/EHLO
	if err = client.Hello("verify.mailnexy.com"); err != nil {
		return &VerificationResult{
			Email:        email,
			Status:       "unknown",
			Details:      "HELO failed: " + err.Error(),
			IsBounceRisk: true,
		}, nil
	}

	// 2. Check if server supports TLS (optional)
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(nil); err != nil {
			return &VerificationResult{
				Email:        email,
				Status:       "unknown",
				Details:      "STARTTLS failed: " + err.Error(),
				IsBounceRisk: true,
			}, nil
		}
	}

	// 3. MAIL FROM check
	if err = client.Mail("noreply@mailnexy.com"); err != nil {
		return &VerificationResult{
			Email:        email,
			Status:       "unknown",
			Details:      "MAIL FROM failed: " + err.Error(),
			IsBounceRisk: true,
		}, nil
	}

	// 4. RCPT TO check - this is the key reachability test
	err = client.Rcpt(email)
	if err == nil {
		return &VerificationResult{
			Email:        email,
			Status:       "valid",
			Details:      "Recipient accepted",
			IsReachable:  true,
			IsBounceRisk: false,
		}, nil
	}

	// Analyze error response
	errMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errMsg, "250"):
		// Some servers return 250 even on failure
		return &VerificationResult{
			Email:        email,
			Status:       "catch-all",
			Details:      "Server accepts all emails (catch-all)",
			IsReachable:  true,
			IsBounceRisk: false,
		}, nil
	case strings.Contains(errMsg, "550"):
		// Mailbox doesn't exist
		return &VerificationResult{
			Email:        email,
			Status:       "invalid",
			Details:      "Mailbox doesn't exist",
			IsReachable:  false,
			IsBounceRisk: true,
		}, nil
	case strings.Contains(errMsg, "421"), strings.Contains(errMsg, "450"), strings.Contains(errMsg, "451"):
		// Temporary failures
		return &VerificationResult{
			Email:        email,
			Status:       "unknown",
			Details:      "Temporary failure: " + err.Error(),
			IsReachable:  false,
			IsBounceRisk: true,
		}, nil
	default:
		return &VerificationResult{
			Email:        email,
			Status:       "unknown",
			Details:      "SMTP error: " + err.Error(),
			IsReachable:  false,
			IsBounceRisk: true,
		}, nil
	}
}

func loadDisposableDomains() map[string]bool {
	// Load from embedded data or file
	domains := make(map[string]bool)
	for _, d := range strings.Split(disposableDomainList, "\n") {
		d = strings.TrimSpace(d)
		if d != "" {
			domains[d] = true
		}
	}
	return domains
}

const disposableDomainList = `
mailinator.com
tempmail.org
10minutemail.com
guerrillamail.com
trashmail.com
temp-mail.org
yopmail.com
maildrop.cc
dispostable.com
fakeinbox.com
throwawaymail.com
mailnesia.com
getairmail.com
mytemp.email
temp-mail.io
fake-mail.com
mail-temp.com
tempail.com
tempomail.fr
tempinbox.com
tempmailaddress.com
mailmetrash.com
trashmail.net
discard.email
mailcatch.com
tempemail.net
mailinator2.com
mintemail.com
notmailinator.com
spamgourmet.com
spamhole.com
spam.la
spamspot.com
spambox.us
spamfree24.org
spamfree.eu
spam4.me
spamdecoy.net
spamcorptastic.com
spamday.com
spamherelots.com
spamhereplease.com
spamthis.co.uk
spamthisplease.com
suremail.info
thisisnotmyrealemail.com
temporaryinbox.com
thankyou2010.com
trash-mail.at
trash-mail.com
trash-mail.de
trashmail.at
trashmail.com
trashmail.de
trashmail.me
trashmail.net
trashmail.org
trashmail.ws
trashymail.com
trashymail.net
trialmail.de
tyldd.com
wh4f.org
willselfdestruct.com
wronghead.com
www.e4ward.com
zippymail.info
zoemail.org
0-mail.com
0815.ru
0clickemail.com
0wnd.net
0wnd.org
10minutemail.co.za
10minutemail.com
123-m.com
1fsdfdsfsdf.tk
1pad.de
20minutemail.com
21cn.com
2fdgdfgdfgdf.tk
2prong.com
30minutemail.com
33mail.com
3d-painting.com
4gfdsgfdgfd.tk
4warding.com
4warding.net
4warding.org
5ghgfhfghfgh.tk
60minutemail.com
675hosting.com
675hosting.net
675hosting.org
6hjgjhgkilkj.tk
6ip.us
6paq.com
6url.com
75hosting.com
75hosting.net
75hosting.org
7tags.com
9ox.net
a-bc.net
afrobacon.com
agedmail.com
ajaxapp.net
amilegit.com
amiri.net
amiriindustries.com
anonbox.net
anonmails.de
anonymbox.com
antichef.com
antichef.net
antireg.ru
antispam.de
antispam24.de
antispammail.de
armyspy.com
artman-conception.com
azmeil.tk
baxomale.ht.cx
beefmilk.com
bigstring.com
binkmail.com
bio-muesli.net
bobmail.info
bodhi.lawlita.com
bofthew.com
bootybay.de
boun.cr
bouncr.com
breakthru.com
brefmail.com
broadbandninja.com
bsnow.net
bspamfree.org
bugmenot.com
bumpymail.com
casualdx.com
centermail.com
centermail.net
chogmail.com
choicemail1.com
clixser.com
cool.fr.nf
courriel.fr.nf
courrieltemporaire.com
cubiclink.com
curryworld.de
cust.in
dacoolest.com
dandikmail.com
dayrep.com
deadaddress.com
deadspam.com
delikkt.de
despam.it
despammed.com
devnullmail.com
dfgh.net
digitalsanctuary.com
discardmail.com
discardmail.de
disposableaddress.com
disposableemailaddresses.com
disposableinbox.com
dispose.it
dispostable.com
dodgeit.com
dodgit.com
dodgit.org
donemail.ru
dontreg.com
dontsendmespam.de
dump-email.info
dumpandjunk.com
dumpmail.de
dumpyemail.com
e-mail.com
e-mail.org
e4ward.com
email60.com
emaildienst.de
emailigo.de
emailinfive.com
emailmiser.com
emailsensei.com
emailtemporario.com.br
emailwarden.com
emailx.at.hm
emailxfer.com
emeil.in
emeil.ir
emz.net
enterto.com
ephemail.net
etranquil.com
etranquil.net
etranquil.org
explodemail.com
fakeinbox.com
fakeinformation.com
fansworldwide.de
fantasymail.de
fightallspam.com
filzmail.com
fivemail.de
fleckens.hu
frapmail.com
friendlymail.co.uk
fuckingduh.com
fudgerub.com
fyii.de
garliclife.com
gehensiemirnichtaufdensack.de
get1mail.com
get2mail.fr
getonemail.com
ghosttexter.de
giantmail.de
girlsundertheinfluence.com
gishpuppy.com
gmial.com
goemailgo.com
gotmail.net
gotmail.org
gotti.otherinbox.com
great-host.in
greensloth.com
gsrv.co.uk
guerillamail.biz
guerillamail.com
guerillamail.net
guerillamail.org
guerrillamail.biz
guerrillamail.com
guerrillamail.de
guerrillamail.info
guerrillamail.net
guerrillamail.org
guerrillamailblock.com
gustr.com
h.mintemail.com
h8s.org
haltospam.com
harakirimail.com
hat-geld.de
herp.in
hidemail.de
hidzz.com
hmamail.com
hochsitze.com
hotpop.com
hulapla.de
ieatspam.eu
ieatspam.info
ihateyoualot.info
iheartspam.org
imails.info
inboxclean.com
inboxclean.org
incognitomail.com
incognitomail.net
incognitomail.org
insorg-mail.info
ipoo.org
irish2me.com
iwi.net
jetable.com
jetable.fr.nf
jetable.net
jetable.org
jnxjn.com
junk1e.com
kasmail.com
kaspop.com
killmail.com
killmail.net
klassmaster.com
klassmaster.net
klzlk.com
knol-power.nl
kulturbetrieb.info
kurzepost.de
letthemeatspam.com
lhsdv.com
lifebyfood.com
link2mail.net
litedrop.com
lol.ovpn.to
lookugly.com
lopl.co.cc
lr78.com
m4ilweb.info
maboard.com
mail-temporaire.fr
mail.by
mail.mezimages.net
mail.zp.ua
mail1a.de
mail21.cc
mail2rss.org
mail333.com
mail4trash.com
mailbidon.com
mailbiz.biz
mailblocks.com
mailbucket.org
mailcat.biz
mailcatch.com
mailde.de
mailde.info
maildrop.cc
maildu.de
maildx.com
maileater.com
mailexpire.com
mailfa.tk
mailforspam.com
mailfreeonline.com
mailguard.me
mailimate.com
mailin8r.com
mailinater.com
mailinator.com
mailinator.net
mailinator.org
mailinator2.com
mailincubator.com
mailismagic.com
mailme.ir
mailme.lv
mailmetrash.com
mailmoat.com
mailms.com
mailnator.com
mailnesia.com
mailnull.com
mailorg.org
mailpick.biz
mailproxsy.com
mailquack.com
mailrock.biz
mailsac.com
mailscrap.com
mailseal.de
mailshell.com
mailsiphon.com
mailslapping.com
mailslite.com
mailtemp.info
mailtome.de
mailtrash.net
mailtv.net
mailtv.tv
mailzilla.com
mailzilla.org
mbx.cc
mega.zik.dj
meinspamschutz.de
meltmail.com
messagebeamer.de
mezimages.net
mierdamail.com
mintemail.com
moburl.com
moncourrier.fr.nf
monemail.fr.nf
monmail.fr.nf
msa.minsmail.com
mt2009.com
mt2014.com
mx0.wwwnew.eu
mycleaninbox.net
mypartyclip.de
myphantomemail.com
mysamp.de
mytempemail.com
mytempmail.com
mytrashmail.com
neomailbox.com
nepwk.com
nervmich.net
nervtmich.net
netmails.com
netmails.net
netzidiot.de
neverbox.com
nice-4u.com
nincsmail.com
nnh.com
no-spam.ws
nobulk.com
noclickemail.com
nogmailspam.info
nomail.xl.cx
nomail2me.com
nomorespamemails.com
nospam.ze.tc
nospam4.us
nospamfor.us
nospammail.net
notmailinator.com
nowhere.org
nowmymail.com
nurfuerspam.de
nus.edu.sg
nwldx.com
objectmail.com
obobbo.com
odaymail.com
olypmall.ru
oneoffemail.com
onewaymail.com
online.ms
oopi.org
ordinaryamerican.net
otherinbox.com
ourklips.com
outlawspam.com
ovpn.to
owlpic.com
pancakemail.com
pcusers.otherinbox.com
pepbot.com
pfui.ru
pimpedupmyspace.com
pjjkp.com
plexolan.de
politikerclub.de
poofy.org
pookmail.com
privacy.net
proxymail.eu
prtnx.com
punkass.com
putthisinyourspamdatabase.com
qq.com
quickinbox.com
rcpt.at
recode.me
recursor.net
regbypass.com
regbypass.comsafe-mail.net
rejectmail.com
rhyta.com
rmqkr.net
royal.net
rtrtr.com
s0ny.net
safe-mail.net
safersignup.de
safetymail.info
safetypost.de
sandelf.de
saynotospams.com
schafmail.de
schrott-email.de
secretemail.de
secure-mail.biz
selfdestructingmail.com
sendspamhere.com
sharklasers.com
shieldedmail.com
shiftmail.com
shitmail.me
shitware.nl
shmeriously.com
shortmail.net
sibmail.com
sinnlos-mail.de
slapsfromlastnight.com
slaskpost.se
smellfear.com
snakemail.com
sneakemail.com
snkmail.com
sofimail.com
sofort-mail.de
sogetthis.com
soodonims.com
spam.la
spam.su
spam4.me
spamavert.com
spambob.com
spambob.net
spambob.org
spambog.com
spambog.de
spambog.net
spambog.ru
spambooger.com
spambox.info
spambox.irishspringrealty.com
spambox.us
spamcannon.com
spamcannon.net
spamcero.com
spamcon.org
spamcorptastic.com
spamcowboy.com
spamcowboy.net
spamcowboy.org
spamday.com
spamex.com
spamfree.eu
spamfree24.com
spamfree24.de
spamfree24.eu
spamfree24.info
spamfree24.net
spamfree24.org
spamgourmet.com
spamherelots.com
spamhereplease.com
spamhole.com
spamify.com
spaminator.de
spamkill.info
spaml.com
spaml.de
spammotel.com
spamobox.com
spamoff.de
spamsalad.in
spamslicer.com
spamspot.com
spamstack.net
spamthis.co.uk
spamthisplease.com
spamtrail.com
speed.1s.fr
spikio.com
spoofmail.de
stuffmail.de
supergreatmail.com
supermailer.jp
suremail.info
teewars.org
teleworm.com
teleworm.us
tempalias.com
tempe-mail.com
tempemail.biz
tempemail.com
tempemail.net
tempinbox.co.uk
tempinbox.com
tempmail.it
tempmail2.com
tempmaildemo.com
tempmailer.com
tempmailer.de
tempomail.fr
temporarily.de
temporarioemail.com.br
temporaryemail.net
temporaryforwarding.com
temporaryinbox.com
thanksnospam.info
thankyou2010.com
thismail.net
throwawayemailaddress.com
tilien.com
tmailinator.com
tradermail.info
trash-amil.com
trash-mail.at
trash-mail.com
trash-mail.de
trash2009.com
trashdevil.com
trashdevil.de
trashemail.de
trashmail.at
trashmail.com
trashmail.de
trashmail.me
trashmail.net
trashmail.org
trashmail.ws
trashmailer.com
trashymail.com
trashymail.net
trialmail.de
trillianpro.com
turual.com
twinmail.de
tyldd.com
uggsrock.com
upliftnow.com
uplipht.com
venompen.com
veryrealemail.com
viditag.com
vipmail.name
vipmail.pw
vpn.st
vsimcard.com
vubby.com
wasteland.rfc822.org
webemail.me
weg-werf-email.de
wegwerf-emails.de
wegwerfadresse.de
wegwerfemail.com
wegwerfemail.de
wegwerfmail.de
wegwerfmail.info
wegwerfmail.net
wegwerfmail.org
wh4f.org
whyspam.me
willselfdestruct.com
winemaven.info
wronghead.com
wuzup.net
wuzupmail.net
www.e4ward.com
www.gishpuppy.com
www.mailinator.com
wwwnew.eu
xagloo.com
xemaps.com
xents.com
xmaily.com
xoxy.net
yep.it
yogamaven.com
yopmail.com
yopmail.fr
yopmail.net
youmailr.com
yourdomain.com
yourlifesucks.cu.cc
yuurok.com
zehnminutenmail.de
zippymail.info
zoemail.net
zoemail.org
0-mail.com
0815.ru
0clickemail.com
0wnd.net
0wnd.org
10minutemail.co.za
10minutemail.com
123-m.com
1fsdfdsfsdf.tk
1pad.de
20minutemail.com
21cn.com
2fdgdfgdfgdf.tk
2prong.com
30minutemail.com
33mail.com
3d-painting.com
4gfdsgfdgfd.tk
4warding.com
4warding.net
4warding.org
5ghgfhfghfgh.tk
60minutemail.com
675hosting.com
675hosting.net
675hosting.org
6hjgjhgkilkj.tk
6ip.us
6paq.com
6url.com
75hosting.com
75hosting.net
75hosting.org
7tags.com
9ox.net
a-bc.net
afrobacon.com
agedmail.com
ajaxapp.net
amilegit.com
amiri.net
amiriindustries.com
anonbox.net
anonmails.de
anonymbox.com
antichef.com
antichef.net
antireg.ru
antispam.de
antispam24.de
antispammail.de
armyspy.com
artman-conception.com
azmeil.tk
baxomale.ht.cx
beefmilk.com
bigstring.com
binkmail.com
bio-muesli.net
bobmail.info
bodhi.lawlita.com
bofthew.com
bootybay.de
boun.cr
bouncr.com
breakthru.com
brefmail.com
broadbandninja.com
bsnow.net
bspamfree.org
bugmenot.com
bumpymail.com
casualdx.com
centermail.com
centermail.net
chogmail.com
choicemail1.com
clixser.com
cool.fr.nf
courriel.fr.nf
courrieltemporaire.com
cubiclink.com
curryworld.de
cust.in
dacoolest.com
dandikmail.com
dayrep.com
deadaddress.com
deadspam.com
delikkt.de
despam.it
despammed.com
devnullmail.com
dfgh.net
digitalsanctuary.com
discardmail.com
discardmail.de
disposableaddress.com
disposableemailaddresses.com
disposableinbox.com
dispose.it
dispostable.com
dodgeit.com
dodgit.com
dodgit.org
donemail.ru
dontreg.com
dontsendmespam.de
dump-email.info
dumpandjunk.com
dumpmail.de
dumpyemail.com
e-mail.com
e-mail.org
e4ward.com
email60.com
emaildienst.de
emailigo.de
emailinfive.com
emailmiser.com
emailsensei.com
emailtemporario.com.br
emailwarden.com
emailx.at.hm
emailxfer.com
emeil.in
emeil.ir
emz.net
enterto.com
ephemail.net
etranquil.com
etranquil.net
etranquil.org
explodemail.com
fakeinbox.com
fakeinformation.com
fansworldwide.de
fantasymail.de
fightallspam.com
filzmail.com
fivemail.de
fleckens.hu
frapmail.com
friendlymail.co.uk
fuckingduh.com
fudgerub.com
fyii.de
garliclife.com
gehensiemirnichtaufdensack.de
get1mail.com
get2mail.fr
getonemail.com
ghosttexter.de
giantmail.de
girlsundertheinfluence.com
gishpuppy.com
gmial.com
goemailgo.com
gotmail.net
gotmail.org
gotti.otherinbox.com
great-host.in
greensloth.com
gsrv.co.uk
guerillamail.biz
guerillamail.com
guerillamail.net
guerillamail.org
guerrillamail.biz
guerrillamail.com
guerrillamail.de
guerrillamail.info
guerrillamail.net
guerrillamail.org
guerrillamailblock.com
gustr.com
h.mintemail.com
h8s.org
haltospam.com
harakirimail.com
hat-geld.de
herp.in
hidemail.de
hidzz.com
hmamail.com
hochsitze.com
hotpop.com
hulapla.de
ieatspam.eu
ieatspam.info
ihateyoualot.info
iheartspam.org
imails.info
inboxclean.com
inboxclean.org
incognitomail.com
incognitomail.net
incognitomail.org
insorg-mail.info
ipoo.org
irish2me.com
iwi.net
jetable.com
jetable.fr.nf
jetable.net
jetable.org
jnxjn.com
junk1e.com
kasmail.com
kaspop.com
killmail.com
killmail.net
klassmaster.com
klassmaster.net
klzlk.com
knol-power.nl
kulturbetrieb.info
kurzepost.de
letthemeatspam.com
lhsdv.com
lifebyfood.com
link2mail.net
litedrop.com
lol.ovpn.to
lookugly.com
lopl.co.cc
lr78.com
m4ilweb.info
maboard.com
mail-temporaire.fr
mail.by
mail.mezimages.net
mail.zp.ua
mail1a.de
mail21.cc
mail2rss.org
mail333.com
mail4trash.com
mailbidon.com
mailbiz.biz
mailblocks.com
mailbucket.org
mailcat.biz
mailcatch.com
mailde.de
mailde.info
maildrop.cc
maildu.de
maildx.com
maileater.com
mailexpire.com
mailfa.tk
mailforspam.com
mailfreeonline.com
mailguard.me
mailimate.com
mailin8r.com
mailinater.com
mailinator.com
mailinator.net
mailinator.org
mailinator2.com
mailincubator.com
mailismagic.com
mailme.ir
mailme.lv
mailmetrash.com
mailmoat.com
mailms.com
mailnator.com
mailnesia.com
mailnull.com
mailorg.org
mailpick.biz
mailproxsy.com
mailquack.com
mailrock.biz
mailsac.com
mailscrap.com
mailseal.de
mailshell.com
mailsiphon.com
mailslapping.com
mailslite.com
mailtemp.info
mailtome.de
mailtrash.net
mailtv.net
mailtv.tv
mailzilla.com
mailzilla.org
mbx.cc
mega.zik.dj
meinspamschutz.de
meltmail.com
messagebeamer.de
mezimages.net
mierdamail.com
mintemail.com
moburl.com
moncourrier.fr.nf
monemail.fr.nf
monmail.fr.nf
msa.minsmail.com
mt2009.com
mt2014.com
mx0.wwwnew.eu
mycleaninbox.net
mypartyclip.de
myphantomemail.com
mysamp.de
mytempemail.com
mytempmail.com
mytrashmail.com
neomailbox.com
nepwk.com
nervmich.net
nervtmich.net
netmails.com
netmails.net
netzidiot.de
neverbox.com
nice-4u.com
nincsmail.com
nnh.com
no-spam.ws
nobulk.com
noclickemail.com
nogmailspam.info
nomail.xl.cx
nomail2me.com
nomorespamemails.com
nospam.ze.tc
nospam4.us
nospamfor.us
nospammail.net
notmailinator.com
nowhere.org
nowmymail.com
nurfuerspam.de
nus.edu.sg
nwldx.com
objectmail.com
obobbo.com
odaymail.com
olypmall.ru
oneoffemail.com
onewaymail.com
online.ms
oopi.org
ordinaryamerican.net
otherinbox.com
ourklips.com
outlawspam.com
ovpn.to
owlpic.com
pancakemail.com
pcusers.otherinbox.com
pepbot.com
pfui.ru
pimpedupmyspace.com
pjjkp.com
plexolan.de
politikerclub.de
poofy.org
pookmail.com
privacy.net
proxymail.eu
prtnx.com
punkass.com
putthisinyourspamdatabase.com
qq.com
quickinbox.com
rcpt.at
recode.me
recursor.net
regbypass.com
regbypass.comsafe-mail.net
rejectmail.com
rhyta.com
rmqkr.net
royal.net
rtrtr.com
s0ny.net
safe-mail.net
safersignup.de
safetymail.info
safetypost.de
sandelf.de
saynotospams.com
schafmail.de
schrott-email.de
secretemail.de
secure-mail.biz
selfdestructingmail.com
sendspamhere.com
sharklasers.com
shieldedmail.com
shiftmail.com
shitmail.me
shitware.nl
shmeriously.com
shortmail.net
sibmail.com
sinnlos-mail.de
slapsfromlastnight.com
slaskpost.se
smellfear.com
snakemail.com
sneakemail.com
snkmail.com
sofimail.com
sofort-mail.de
sogetthis.com
soodonims.com
spam.la
spam.su
spam4.me
spamavert.com
spambob.com
spambob.net
spambob.org
spambog.com`

// // utils/verifier.go
// package utils

// import (
// 	"context"
// 	"fmt"
// 	"net"
// 	"net/smtp"
// 	"regexp"
// 	"strings"
// 	"sync"
// 	"time"
// )

// type VerificationResult struct {
// 	Email        string `json:"email"`
// 	Status       string `json:"status"` // valid, invalid, disposable, catch-all, unknown
// 	Details      string `json:"details"`
// 	IsReachable  bool   `json:"is_reachable"`
// 	IsBounceRisk bool   `json:"is_bounce_risk"`
// }

// var (
// 	// Expanded disposable domains (500+ domains)
// 	disposableDomains = loadDisposableDomains()

// 	// Major free email providers
// 	freeEmailProviders = []string{
// 		"gmail.com", "yahoo.com", "outlook.com", "hotmail.com",
// 		"aol.com", "protonmail.com", "icloud.com", "mail.com",
// 		"yandex.com", "zoho.com", "gmx.com",
// 	}

// 	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// 	// Common email typos
// 	commonTypos = map[string]string{
// 		"gmai.com":   "gmail.com",
// 		"gmal.com":   "gmail.com",
// 		"gmail.co":   "gmail.com",
// 		"yaho.com":   "yahoo.com",
// 		"hotmai.com": "hotmail.com",
// 		"outlok.com": "outlook.com",
// 	}

// 	// Domain to MX cache
// 	mxCache = struct {
// 		sync.RWMutex
// 		m map[string][]*net.MX
// 	}{m: make(map[string][]*net.MX)}
// )

// func loadDisposableDomains() map[string]bool {
// 	// Load from embedded data or file
// 	domains := make(map[string]bool)
// 	for _, d := range strings.Split(disposableDomainList, "\n") {
// 		d = strings.TrimSpace(d)
// 		if d != "" {
// 			domains[d] = true
// 		}
// 	}
// 	return domains
// }

// const disposableDomainList = `
// mailinator.com
// tempmail.org
// 10minutemail.com
// guerrillamail.com
// trashmail.com
// temp-mail.org
// yopmail.com
// maildrop.cc
// dispostable.com
// fakeinbox.com
// ... [500+ more domains] ...
// `

// func VerifyEmailAddress(email string) (*VerificationResult, error) {
// 	email = strings.ToLower(strings.TrimSpace(email))
// 	result := &VerificationResult{
// 		Email:        email,
// 		Status:       "unknown",
// 		IsReachable:  false,
// 		IsBounceRisk: true,
// 	}

// 	// 1. Basic syntax validation
// 	if !emailRegex.MatchString(email) {
// 		result.Status = "invalid"
// 		result.Details = "Invalid email format"
// 		return result, nil
// 	}

// 	parts := strings.Split(email, "@")
// 	if len(parts) != 2 {
// 		result.Status = "invalid"
// 		result.Details = "Invalid email format"
// 		return result, nil
// 	}

// 	localPart, domain := parts[0], parts[1]

// 	// 2. Check for common typos
// 	if suggestedDomain, ok := commonTypos[domain]; ok {
// 		result.Status = "invalid"
// 		result.Details = fmt.Sprintf("Possible typo, did you mean %s@%s?", localPart, suggestedDomain)
// 		return result, nil
// 	}

// 	// 3. Disposable email check
// 	if isDisposableDomain(domain) {
// 		result.Status = "disposable"
// 		result.Details = "Disposable email domain"
// 		return result, nil
// 	}

// 	// 4. DNS/MX record check
// 	mxRecords, err := getMXRecords(domain)
// 	if err != nil || len(mxRecords) == 0 {
// 		result.Status = "invalid"
// 		result.Details = "Domain has no MX records"
// 		return result, nil
// 	}

// 	// 5. Enhanced SMTP verification
// 	return verifySMTP(domain, email, mxRecords)
// }

// func isDisposableDomain(domain string) bool {
// 	return disposableDomains[domain]
// }

// func getMXRecords(domain string) ([]*net.MX, error) {
// 	// Check cache first
// 	mxCache.RLock()
// 	if records, ok := mxCache.m[domain]; ok {
// 		mxCache.RUnlock()
// 		return records, nil
// 	}
// 	mxCache.RUnlock()

// 	// Lookup fresh records with timeout
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	var resolver net.Resolver
// 	mxRecords, err := resolver.LookupMX(ctx, domain)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Update cache
// 	mxCache.Lock()
// 	mxCache.m[domain] = mxRecords
// 	mxCache.Unlock()

// 	return mxRecords, nil
// }

// func verifySMTP(domain, email string, mxRecords []*net.MX) (*VerificationResult, error) {
// 	result := &VerificationResult{
// 		Email:        email,
// 		Status:       "unknown",
// 		IsReachable:  false,
// 		IsBounceRisk: true,
// 	}

// 	// Try multiple MX servers
// 	for _, mx := range mxRecords {
// 		mailServer := strings.TrimSuffix(mx.Host, ".")

// 		// Try common ports - removed the unused port declaration
// 		portsToTry := []string{"25", "587", "465"}
// 		if isFreeEmailProvider(domain) {
// 			// For free providers, try submission ports first
// 			portsToTry = []string{"587", "465", "25"}
// 		}

// 		for _, port := range portsToTry {
// 			addr := fmt.Sprintf("%s:%s", mailServer, port)
// 			smtpResult, err := checkSMTP(addr, domain, email)
// 			if err == nil {
// 				return smtpResult, nil
// 			}
// 		}
// 	}

// 	result.Details = "All SMTP verification attempts failed"
// 	return result, nil
// }

// func isFreeEmailProvider(domain string) bool {
// 	for _, provider := range freeEmailProviders {
// 		if domain == provider {
// 			return true
// 		}
// 	}
// 	return false
// }

// func checkSMTP(addr, domain, email string) (*VerificationResult, error) {
// 	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer conn.Close()

// 	client, err := smtp.NewClient(conn, domain)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer client.Close()

// 	// Set timeout for each SMTP command
// 	deadline := time.Now().Add(15 * time.Second)
// 	conn.SetDeadline(deadline)

// 	// 1. Send HELO/EHLO
// 	if err = client.Hello("verify.example.com"); err != nil {
// 		return &VerificationResult{
// 			Email:        email,
// 			Status:       "unknown",
// 			Details:      "HELO failed: " + err.Error(),
// 			IsBounceRisk: true,
// 		}, nil
// 	}

// 	// 2. Check if server supports TLS (optional)
// 	if ok, _ := client.Extension("STARTTLS"); ok {
// 		if err = client.StartTLS(nil); err != nil {
// 			return &VerificationResult{
// 				Email:        email,
// 				Status:       "unknown",
// 				Details:      "STARTTLS failed: " + err.Error(),
// 				IsBounceRisk: true,
// 			}, nil
// 		}
// 	}

// 	// 3. MAIL FROM check
// 	if err = client.Mail("sender@example.com"); err != nil {
// 		return &VerificationResult{
// 			Email:        email,
// 			Status:       "unknown",
// 			Details:      "MAIL FROM failed: " + err.Error(),
// 			IsBounceRisk: true,
// 		}, nil
// 	}

// 	// 4. RCPT TO check - this is the key reachability test
// 	err = client.Rcpt(email)
// 	if err == nil {
// 		return &VerificationResult{
// 			Email:        email,
// 			Status:       "valid",
// 			Details:      "Recipient accepted",
// 			IsReachable:  true,
// 			IsBounceRisk: false,
// 		}, nil
// 	}

// 	// Analyze error response
// 	errMsg := strings.ToLower(err.Error())
// 	switch {
// 	case strings.Contains(errMsg, "250"):
// 		// Some servers return 250 even on failure
// 		return &VerificationResult{
// 			Email:        email,
// 			Status:       "catch-all",
// 			Details:      "Server accepts all emails (catch-all)",
// 			IsReachable:  true,
// 			IsBounceRisk: false,
// 		}, nil
// 	case strings.Contains(errMsg, "550"):
// 		// Mailbox doesn't exist
// 		return &VerificationResult{
// 			Email:        email,
// 			Status:       "invalid",
// 			Details:      "Mailbox doesn't exist",
// 			IsReachable:  false,
// 			IsBounceRisk: true,
// 		}, nil
// 	default:
// 		return &VerificationResult{
// 			Email:        email,
// 			Status:       "unknown",
// 			Details:      "SMTP error: " + err.Error(),
// 			IsReachable:  false,
// 			IsBounceRisk: true,
// 		}, nil
// 	}
// }
