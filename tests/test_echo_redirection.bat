set "CMD_FILE=test_out.txt"
set "LST=LISTING"
echo %LST%;                        > %CMD_FILE%  Listing file name
echo Second line >> %CMD_FILE%  Comment here
type %CMD_FILE%
del %CMD_FILE%
