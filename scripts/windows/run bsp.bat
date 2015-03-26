@rem You must already have copied the id1 and qdqr-recam directories from your quake install
@rem and the retextures listed below.
c:
cd \go\qpov
mkdir test-lights
bsp -pak id1/pak0.pak,id1/pak1.pak,qdqr-recam/pak0.pak convert -lights=true -textures=true -out test-lights -retexture=c:\go\qpov\retexture\maps
pause
