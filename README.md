# smtp-client-simple
A very simple exposure of a given smtp-servers protocol through http.

The api accepts no path variables. Only query parameters: `from`, `to`(one or more).

The server additionally takes `subject` and `msg` query parameters.
And will add any http body content, as the email body. 

##example
usage (caller side):
```shell script
curl 'some-host:<port>?to=some@mail.com&some-other@mail.com&from=me@some.com&msg=some%20body&subject=Hello%20World'
```
with message from file:
```shell script
$ cat <<EOF > message
Subject: whatever you want

body after two linebreaks
EOF
$ curl some-host:<port>?to=some@mail.com --data-binary @message
```
usage docker for a server exposing the smtp-server `smtp.my.domain.com:25` without tls from the port `12345`, always sending from the mail `notifier@domain.com`
```shell script
docker run --rm -d -p 8080:12345 mjuul/smtp-client-simple --port 12345 --smtp-server smtp.my.domain.com:25 --forced-from notifier@domain.com --skip-tls
```
get help text
```shell script
docker run mjuul/smtp-client-simple --help
```
the server allows configuration of default fallbacks for, `body`, `subject`, `to`(plural via comma-separation) and `from`.
## use case
I use it in pipelines exposing an internal smtp-server. For custom notifications.

## TODO 
* insecure / log-in - including a locked address?
