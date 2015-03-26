set GOPATH=c:\go\qpov
c:
mkdir c:\go\qpov
cd \go\qpov
go build github.com/ThomasHabets/qpov/cmd/mdl
go build github.com/ThomasHabets/qpov/cmd/dem
go build github.com/ThomasHabets/qpov/cmd/bsp
go build github.com/ThomasHabets/qpov/cmd/render
pause
