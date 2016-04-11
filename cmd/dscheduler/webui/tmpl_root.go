// -*- html -*-
package main

const rootTmpl = `
{{$root := .}}
<html>
  <head>
    <title>QPov</title>
    <meta name="google-signin-scope" content="email">
    <meta name="google-signin-client_id" content="{{.OAuthClientID}}">
    <script src="https://apis.google.com/js/platform.js" async defer></script>
    <script>
    function signOut() {
      var auth2 = gapi.auth2.getAuthInstance();
      auth2.signOut().then(function () {});
      document.cookie = "jwt=;max-age=0";
      location.reload();
    }
    function onSignIn(googleUser) {
      var profile = googleUser.getBasicProfile();
      var img = profile.getImageUrl();
      if (img != undefined) {
        document.getElementById("profile-img").innerHTML = "<img src='"+img+"'>";
      }
      document.getElementById("sign-out").style.display = "inline";
      document.getElementById("profile-email").innerHTML = profile.getEmail();
      document.cookie = "jwt=" + googleUser.getAuthResponse().id_token + ";secure";
      // TODO: do ajax call to exchange for qpov cookie that is httponly, and
      // delete the jwt cookie. The reload can then happen when new login, and
      // do a refresh. Also the jwt cookie is pretty big.
      // location.reload();
    };
  </script>
<style>
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
#gbuttons {
  float: right;
  display: inline-block;
}
#profile-email {
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
  float: right;
}
</style>
  </head>
  <div id="nav">
    QPov
    <div id="gbuttons">
      <div class="g-signin2" data-onsuccess="onSignIn" data-theme="dark"></div>
    </div>
    <span class="top-button" id="sign-out" style="display: none"><a href="#" onclick="signOut();">Sign out</a></span>
    <span id="profile-img"></span>
    <span id="profile-email"></span>
  </div>
  <body>

    {{if .Errors}}
      <h2>Errors while rendering this page:</h2>
      <ul>
        {{range .Errors}}
          <li>{{.}}</li>
        {{end}}
      </ul>
    {{end}}

    <h2>Scheduler stats</h2>
    {{if .Stats.SchedulingStats}}
      <table>
        <tr><th colspan="2">Orders</th></tr>
        <tr><th>Total</th><td>{{.Stats.SchedulingStats.Orders}}</td></tr>
        <tr><th>Active</th><td>{{.Stats.SchedulingStats.ActiveOrders}}</td></tr>
        <tr><th>Done</th><td>{{.Stats.SchedulingStats.DoneOrders}}</td></tr>
        <tr><th>Unstarted</th><td>{{.UnstartedOrders}}</td></tr>
        <tr><th colspan="2">Leases</th></tr>
        <tr><th>Total</th><td>{{.Stats.SchedulingStats.Leases}}</td></tr>
        <tr><th>Active</th><td>{{.Stats.SchedulingStats.ActiveLeases}}</td></tr>
        <tr><th>Done</th><td>{{.Stats.SchedulingStats.DoneLeases}}</td></tr>
      </table>
    {{end}}

    <h2>Active leases</h2>
    <table>
      <tr>
        <th>Order</th>
        <th>Lease</th>
        <th>Created</th>
        <th>Lifetime</th>
        <th>Updated</th>
        <th>Expires</th>
        <!--  <th>Package</th> -->
        <th>File</th>
        <th>Client</th>
      </tr>
      {{range .Leases}}
        <tr>
          <td nowrap class="fixed">{{.OrderId}}</td>
          <td nowrap class="fixed"><a href="{{$root.Root}}/lease/{{.LeaseId}}">{{.LeaseId}}</a></td>
          <td nowrap>{{.CreatedMs|fmsdate "2006-01-02 15:04"}}</td>
          <td nowrap>{{.CreatedMs|fmssince}}</td>
          <td nowrap>{{.UpdatedMs|fmssince}}</td>
          <td nowrap>{{.ExpiresMs|fmsuntil}}</td>
          <!--  <td nowrap>{{.Order.Package|fileonly}}</td> -->
          <td nowrap>{{.Order.File}}</td>
          <td nowrap>{{.Address}} {{.Hostname}}</td>
        </tr>
      {{end}}
    </table>

    <h2>Finished</h2>
    <table>
      <tr>
        <th>Order</th>
        <th>Lease</th>
        <th>Created</th>
        <th>Done</th>
        <th>Time</th>
        <th>Image</th>
        <!--  <th>Package</th> -->
        <th>File</th>
        <!--  <th>Args</th> -->
        <th>Client</th>
      </tr>
      {{range .DoneLeases}}
        <tr>
          <td nowrap class="fixed">{{.OrderId}}</td>
          <td nowrap class="fixed"><a href="{{$root.Root}}/lease/{{.LeaseId}}">{{.LeaseId}}</a></td>
          <td nowrap>{{.CreatedMs|fmsdate "2006-01-02 15:04"}}</td>
          <td nowrap>{{.UpdatedMs|fmsdate "2006-01-02 15:04"}}</td>
          <td nowrap>{{.CreatedMs|fmssub .UpdatedMs}}</td>
          <td nowrap><a href="{{$root.Root}}/image/{{.LeaseId}}">Image</a></td>
          <!--  <td nowrap>{{.Order.Package|fileonly}}</td> -->
          <td nowrap>{{.Order.File}}</td>
          <td nowrap>{{.Address}} {{.Hostname}}</td>
          <!--  <td nowrap>{{.Order.Args}}</td> -->
        </tr>
      {{end}}
    </table>

    <hr>
    Page server time: {{.PageTime}}
  </body>
</html>
`
