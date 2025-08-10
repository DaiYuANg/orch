//go:build darwin
// +build darwin

package pkg

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>

const char* GetIOPlatformUUID() {
    CFStringRef serial;
    io_service_t platformExpert = IOServiceGetMatchingService(kIOMasterPortDefault,
                                  IOServiceMatching("IOPlatformExpertDevice"));
    if (!platformExpert) {
        return NULL;
    }
    serial = IORegistryEntryCreateCFProperty(platformExpert, CFSTR("IOPlatformUUID"),
                                             kCFAllocatorDefault, 0);
    IOObjectRelease(platformExpert);

    if (!serial) {
        return NULL;
    }
    const char *uuid = CFStringGetCStringPtr(serial, kCFStringEncodingUTF8);
    CFRelease(serial);
    return uuid;
}
*/
import "C"

func sysMachineId() string {
	uuid := C.GetIOPlatformUUID()
	if uuid != nil {
		return C.GoString(uuid)
	}
	return ""
}
