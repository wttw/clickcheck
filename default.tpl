From: Whoever <{{ .From }}>
To: Me <{{ .To }}>
Date: {{ .Date }}
Subject: I'm a test email
MIME-Version: 1.0
Content-Type: text/html

<html><body>
<p>I'm a little teapot.</p>
<p>Short and stout.</p>
<p>Here's my <a href="link">{{ .Link }}</a></p>
<p>Link is {{ .Link }}</p>
<p>{{ .Note | html }}</p>
</body></html>
