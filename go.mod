module github.com/knusbaum/duitchat

go 1.11

replace github.com/mjl-/duit => github.com/knusbaum/duit v0.0.0-20200413214450-5cc3648b5133

replace 9fans.net/go => github.com/knusbaum/go v0.0.0-20200413212707-848f58a0ec6e

require (
	9fans.net/go v0.0.2
	github.com/fsnotify/fsnotify v1.4.9
	github.com/mjl-/duit v0.0.0-20200330125617-580cb0b2843f
)
