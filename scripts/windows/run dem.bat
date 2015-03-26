@rem You must already have copied the id1 and qdqr-recam directories from your quake install
c:
cd \go\qpov
mkdir test-lights\e1m1
mkdir test-lights\e1m2
mkdir test-lights\e1m3
mkdir test-lights\e1m4
mkdir test-lights\e1m5
mkdir test-lights\e1m6
mkdir test-lights\e1m7
mkdir test-lights\e2m1
mkdir test-lights\e2m2
mkdir test-lights\e2m3
mkdir test-lights\e2m4
mkdir test-lights\e2m5
mkdir test-lights\e2m6
mkdir test-lights\e2m7
mkdir test-lights\e3m1
mkdir test-lights\e3m2
mkdir test-lights\e3m3
mkdir test-lights\e3m4
mkdir test-lights\e3m5
mkdir test-lights\e3m6
mkdir test-lights\e3m7
mkdir test-lights\e4m1
mkdir test-lights\e4m2
mkdir test-lights\e4m3
mkdir test-lights\e4m4
mkdir test-lights\e4m5
mkdir test-lights\e4m6
mkdir test-lights\e4m7
mkdir test-lights\e4m8
mkdir test-lights\end
mkdir test-lights\start

dem -pak id1/pak0.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e1m1 e1m1.dem
dem -pak id1/pak0.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e1m2 e1m2.dem
dem -pak id1/pak0.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e1m3 e1m3.dem
dem -pak id1/pak0.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e1m4 e1m4.dem
dem -pak id1/pak0.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e1m5 e1m5.dem
dem -pak id1/pak0.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e1m6 e1m6.dem
dem -pak id1/pak0.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e1m7 e1m7.dem

dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e2m1 e2m1.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e2m2 e2m2.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e2m3 e2m3.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e2m4 e2m4.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e2m5 e2m5.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e2m6 e2m6.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e2m7 e2m7.dem

dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e3m1 e3m1.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e3m2 e3m2.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e3m3 e3m3.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e3m4 e3m4.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e3m5 e3m5.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e3m6 e3m6.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e3m7 e3m7.dem

dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e4m1 e4m1.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e4m2 e4m2.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e4m3 e4m3.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e4m4 e4m4.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e4m5 e4m5.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e4m6 e4m6.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e4m7 e4m7.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\e4m8 e4m8.dem

dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\start start.dem
dem -pak id1/pak1.pak,qdqr-recam\PAK0.PAK convert -fps 30 -out test-lights\end end.dem
pause
