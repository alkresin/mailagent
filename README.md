# mailagent
The program checks a mail from a number of mailboxes at specified intervals.
It signals when new messages arrives, show a specified number of last messages, allows to delete some of them.

The Mailagent is written in Golang and have the following dependencies:

 1) go-imap, go-imap/client packages to implement access to the imap mail server: https://godoc.org/github.com/emersion/go-imap
 2) external, a GUI framework, https://github.com/alkresin/external

 You need also a GuiServer executable, which is necessary for the External, see https://github.com/alkresin/guiserver

 Ready binary package for Windows may be downloaded from http://www.kresin.ru/en/guisrv.html

--------------------
Alexander S.Kresin
http://www.kresin.ru/
mailto: alkresin@yahoo.com
