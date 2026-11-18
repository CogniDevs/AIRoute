package hosts

// FlushDNS flushes local OS resolver cache without throwing visible console windows.
func FlushDNS() error {
	return flushPlatformDNS()
}
