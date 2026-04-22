#import <Cocoa/Cocoa.h>
#include "tray.h"

// Exported Go callbacks (defined in tray.go via //export)
extern void goTrayOpen(void);
extern void goTrayQuit(void);
extern void goTrayToggle(void);

@interface LMTrayHandler : NSObject
- (void)open;
- (void)quit;
- (void)toggle;
@end

@implementation LMTrayHandler
- (void)open   { goTrayOpen(); }
- (void)quit   { goTrayQuit(); }
- (void)toggle { goTrayToggle(); }
@end

static NSStatusItem  *gStatusItem  = nil;
static LMTrayHandler *gHandler     = nil;
static NSMenuItem    *gToggleItem  = nil;

void initTray(const char* iconData, int iconLen, int integrationEnabled) {
    // Must run on main thread; Wails has the main loop running at this point.
    dispatch_async(dispatch_get_main_queue(), ^{
        gHandler    = [[LMTrayHandler alloc] init];
        gStatusItem = [[NSStatusBar systemStatusBar]
                        statusItemWithLength:NSVariableStatusItemLength];

        // Icon
        NSData  *data  = [NSData dataWithBytes:iconData length:iconLen];
        NSImage *image = [[NSImage alloc] initWithData:data];
        [image setTemplate:YES]; // auto dark/light mode
        gStatusItem.button.image   = image;
        gStatusItem.button.toolTip = @"LM Bridge";

        // Menu
        NSMenu *menu = [[NSMenu alloc] init];

        NSMenuItem *openItem = [[NSMenuItem alloc]
            initWithTitle:@"Open Dashboard"
                   action:@selector(open)
            keyEquivalent:@""];
        openItem.target = gHandler;
        [menu addItem:openItem];

        [menu addItem:[NSMenuItem separatorItem]];

        gToggleItem = [[NSMenuItem alloc]
            initWithTitle:@"Claude Code Integration"
                   action:@selector(toggle)
            keyEquivalent:@""];
        gToggleItem.target = gHandler;
        gToggleItem.state  = integrationEnabled ? NSControlStateValueOn : NSControlStateValueOff;
        [menu addItem:gToggleItem];

        [menu addItem:[NSMenuItem separatorItem]];

        NSMenuItem *quitItem = [[NSMenuItem alloc]
            initWithTitle:@"Quit LM Bridge"
                   action:@selector(quit)
            keyEquivalent:@"q"];
        quitItem.target = gHandler;
        [menu addItem:quitItem];

        gStatusItem.menu = menu;
    });
}

void updateTrayToggle(int enabled) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gToggleItem) {
            gToggleItem.state = enabled ? NSControlStateValueOn : NSControlStateValueOff;
        }
    });
}
