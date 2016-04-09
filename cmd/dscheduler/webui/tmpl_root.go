// -*- html -*-
package main

const rootTmpl = `
{{$root := .}}
<html>
  <head>
    <title>QPov</title>
    <meta name="google-signin-scope" content="profile email">
    <!-- TODO: Move client ID to cmdline or something -->
    <meta name="google-signin-client_id" content="814877621236-5bji24sq74hqecapr2h13ac07nj7m7ci.apps.googleusercontent.com">
    <script src="https://apis.google.com/js/platform.js" async defer></script>
    <script>
    function signOut() {
      var auth2 = gapi.auth2.getAuthInstance();
      auth2.signOut().then(function () {
      console.log('User signed out.');
      });
    }
    function onSignIn(googleUser) {
    // Useful data for your client-side scripts:
    var profile = googleUser.getBasicProfile();
    console.log("ID: " + profile.getId()); // Don't send this directly to your server!
    console.log('Full Name: ' + profile.getName());
    console.log('Given Name: ' + profile.getGivenName());
    console.log('Family Name: ' + profile.getFamilyName());
    console.log("Image URL: " + profile.getImageUrl());
    console.log("Email: " + profile.getEmail());

    // The ID token you need to pass to your backend:
    var id_token = googleUser.getAuthResponse().id_token;
    console.log("ID Token: " + id_token);
    document.cookie = "jwt=" + id_token;
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
</style>
  </head>
  <div class="g-signin2" data-onsuccess="onSignIn" data-theme="dark"></div>
  <a href="#" onclick="signOut();">Sign out</a>
  <body>
<h1>QPov</h1>

{{if .Errors}}
  <h2>Errors while rendering this page:</h2>
  <ul>
    {{range .Errors}}
      {{.}}
    {{end}}
  </ul>
{{end}}

<h2>Scheduler stats</h2>
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
  <td nowrap>{{.Address}}</td>
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
  <td nowrap>{{.Address}}</td>
<!--  <td nowrap>{{.Order.Args}}</td> -->
</tr>
{{end}}
</table>

<hr>
Page server time: {{.PageTime}}
  </body>
</html>
`
