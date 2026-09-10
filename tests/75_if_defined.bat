@echo off
set V=1
if defined V echo def-yes
set V=
if defined V echo def-still
if not defined V echo def-no
set "W= "
if defined W echo space-defined
echo end
