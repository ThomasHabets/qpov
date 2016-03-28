// -*- html -*-
package main

const rootTmpl = `
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
  <th>Created</th>
  <th>Done</th>
  <th>Time</th>
  <th>Image</th>
<!--  <th>Package</th> -->
  <th>File</th>
<!--  <th>Args</th> -->
  <th>Details</th>
</tr>
{{range .DoneLeases}}
<tr>
  <td nowrap class="fixed">{{.OrderId}}</td>
  <td nowrap>{{.CreatedMs|fmsdate "2006-01-02 15:04"}}</td>
  <td nowrap>{{.UpdatedMs|fmsdate "2006-01-02 15:04"}}</td>
  <td nowrap>{{.CreatedMs|fmssub .UpdatedMs}}</td>
  <td nowrap><a href="/image/{{.LeaseId}}">Image</a></td>
<!--  <td nowrap>{{.Order.Package|fileonly}}</td> -->
  <td nowrap>{{.Order.File}}</td>
  <td nowrap><a href="/lease/{{.LeaseId}}">Details</a></td>
<!--  <td nowrap>{{.Order.Args}}</td> -->
</tr>
{{end}}
</table>

<hr>
Page server time: {{.PageTime}}
`
