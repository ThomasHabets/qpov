// -*- html -*-
package main

const rootTmpl = `
{{ $root := . }}
<style>
  #batches {
    width: 100%;
    white-space: nowrap;
  }
  #batches td.expand {
    width: 90%;
  }
  #batches progress.expand {
    width: calc(100% - 5em);
  }
</style>
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

    <h2>Batches (data periodically updated)</h2>
    <table id="batches">
      <tr>
        <th>Batch</th>
        <th>Started</th>
        <th>Computrons</th>
        <th>Done</th>
        <th>Total</th>
        <th>Completion</th>
      </tr>
      {{range .StatsOverall.BatchStats}}
      <tr>
        <td><a href="/batch/{{.BatchId}}">{{if .Comment}}{{.Comment}}{{else}}{{if .BatchId}}{{.BatchId}}{{else}}none{{end}}{{end}}</a></td>
        <td>{{.Ctime|fsdate "2006-01-02 15:04"}}</td>
        <td>{{.CpuTime.ComputeSeconds}}</td>
        <td>{{.Done}}</td>
        <td>{{.Total}}</td>
        <td class="expand">
          <span style="width: 4em; display: inline-block;">{{fmtpercent .Done .Total}}%</span>
          <progress class="expand" value="{{fmtpercent .Done .Total}}" max="100" />
        </td>
      </tr>
      {{end}}
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
        <th>Address</th>
        <th>Hostname</th>
      </tr>
      {{range .Leases}}
        <tr>
          <td nowrap class="fixed"><a href="{{$root.Root}}/order/{{.OrderId}}">Order</a></td>
          <td nowrap class="fixed">{{if .LeaseId}}<a href="{{$root.Root}}/lease/{{.LeaseId}}">Lease</a>{{end}}</td>
          <td nowrap class="fixed">{{.CreatedMs|fmsdate "2006-01-02 15:04"}}</td>
          <td nowrap class="fixed">{{.CreatedMs|fmssince}}</td>
          <td nowrap class="fixed">{{.UpdatedMs|fmssince}}</td>
          <td nowrap class="fixed">{{.ExpiresMs|fmsuntil}}</td>
          <!--  <td nowrap>{{.Order.Package|fileonly}}</td> -->
          <td nowrap>{{.Order.File}}</td>
          <td nowrap>{{.Address}}</td>
          <td nowrap>{{.Hostname}}</td>
        </tr>
      {{end}}
    </table>
`
