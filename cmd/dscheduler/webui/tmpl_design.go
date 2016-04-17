// -*- html -*-
package main

import (
	"html/template"
)

var tmplDesign = template.Must(template.New("design").Parse(`
{{$root := .}}
<html>
  <head>
    <title>QPov</title>
    <meta name="google-signin-scope" content="email">
    <meta name="google-signin-client_id" content="{{.OAuthClientID}}">
    <script type="text/javascript" src="//ajax.googleapis.com/ajax/libs/jquery/2.0.3/jquery.min.js"></script>
    <script src="https://apis.google.com/js/platform.js" async defer></script>
    <script>
    function signOut() {
      var auth2 = gapi.auth2.getAuthInstance();
      auth2.signOut().then(function () {});
      $.post("{{$root.Root}}/logout", {},
          function() {
            console.log("Logged out OK");
            location.reload();
          }
      );
    }
    function onSignIn(googleUser) {
      var profile = googleUser.getBasicProfile();
      var img = profile.getImageUrl();
      if (img != undefined) {
        document.getElementById("profile-img").innerHTML = "<img src='"+img+"'>";
      }
      document.getElementById("sign-out").style.display = "inline";
      document.getElementById("profile-email").innerHTML = profile.getEmail();
      if ("true" !== document.getElementById("logged-in").dataset.loggedIn) {
        $.post("{{$root.Root}}/login", {"jwt": googleUser.getAuthResponse().id_token},
            function() {
              console.log("Logged in OK");
              location.reload();
          }
        );
      }
    };
  </script>
    <style>
body {
  margin: 0;
}
#content {
  margin-left: 8px;
  margin-right: 8px;
}
.fixed {
  font-family: monospace;
}
table {
  border-collapse: collapse;
}
table, th {
  border: 1px solid black;
}
td {
  text-align: right;
  write-space: nowrap;
  border-right: 1px solid black;
  padding-left: 1em;
  padding-right: 1em;
}
tr:nth-child(odd) {
  background: #EEE
}
#nav {
  font-size: 24px;
  height: 36px;
  width: 100%;
  color: black;
  background-color: lightblue;
}
#nav span {
  margin-left: 8px;
}
#gbuttons {
  float: right;
  display: inline-block;
}
#profile-email {
  margin-top: 0.5em;
  float: right;
  font-size: 14px;
}
#profile-img {
  float: right;
  height: 36px;
  width: 36px;
}
#profile-img img{
  height: 36px;
  width: 36px;
}
.top-button {
  margin-right: 1em;
  float: right;
}
</style>
  </head>
  <div id="nav">
    <span><a href="{{$root.Root}}/">QPov</a></span>
    <span><a href="{{$root.Root}}/stats">Stats</a></span>
    <span><a href="{{$root.Root}}/done">Done</a></span>
    <div id="gbuttons">
      <div class="g-signin2" data-onsuccess="onSignIn" data-theme="dark"></div>
    </div>
    <span class="top-button" id="sign-out" style="display: none"><a href="#" onclick="signOut();">Sign out</a></span>
    <span id="profile-img"></span>
    <span id="profile-email"></span>
  </div>
  <body>
    <input type="hidden" id="logged-in" data-logged-in="{{.LoggedIn}}" />
    {{if .Errors}}
      <h2>Errors while rendering this page:</h2>
      <ul>
        {{range .Errors}}
          <li>{{.}}</li>
        {{end}}
      </ul>
    {{end}}
    <div id="content">{{.Content}}</div>
    <hr>
    Page server time: {{.PageTime}}
  </body>
</html>
`))
