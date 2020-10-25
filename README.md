# smtp-client-simple
A very simple exposure of a given smtp-servers protocol through http.

The api accepts no path variables. Only query parameters: `from`, `to`(one or more).

The server additionally takes `subject` and `msg` query parameters.
And will add any http body content, as the email body. 

## use case
I use it in pipelines exposing our smtp-server. For custom notifications.

## TODO 
* tls
