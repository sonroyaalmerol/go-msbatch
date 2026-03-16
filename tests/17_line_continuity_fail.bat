@echo off
rem Caret mid-word escapes next char (not line continuation)
echo a^&b
rem Multiple carets — each escapes the following character
echo ^e^c^h^o not a command
rem Caret before close paren
echo val^)end
