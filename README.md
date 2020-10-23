# smtp-client-simple
A very simple exposure of a given smtp-servers protocol through http.

The api accepts no path variables. Only query parameters: `from`, `to`(one or more).
Currently it takes `subject` and `body`  query parameters
 I will make it accept only body for the mail `data`. Given as the regular smtp-format.

## use case
I use it in pipelines exposing our smtp-server. For custom notifications.

## TODO 
* tls
