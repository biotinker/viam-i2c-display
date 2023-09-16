viam-i2c-display: *.go */*.go
	# the executable
	go build -o $@ -ldflags "-s -w" -tags osusergo,netgo
	file $@

module.tar.gz: viam-i2c-display
	# the bundled module
	rm -f $@
	tar czf $@ $^
