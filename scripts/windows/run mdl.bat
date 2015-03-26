@rem You must already have copied the id1 and qdqr-recam directories from your quake install
c:
cd \go\qpov
mkdir test-lights
mdl -pak id1/pak0.pak,id1/pak1.pak,qdqr-recam/pak0.pak convert -out test-lights
pause
