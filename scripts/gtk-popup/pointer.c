// Persistent virtual pointer for isolated screenshot tests only.
// Commands on stdin: move X Y, click, quit. Each command acknowledges on stdout.
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <time.h>
#include <wayland-client.h>
#include "virtual-pointer.h"

static struct zwlr_virtual_pointer_manager_v1 *manager;
static void global(void *data, struct wl_registry *registry, uint32_t name,
                   const char *interface, uint32_t version) {
    (void)data; (void)version;
    if (!strcmp(interface, "zwlr_virtual_pointer_manager_v1"))
        manager = wl_registry_bind(registry, name, &zwlr_virtual_pointer_manager_v1_interface, 1);
}
static void removed(void *data, struct wl_registry *registry, uint32_t name) {
    (void)data; (void)registry; (void)name;
}
static const struct wl_registry_listener listener = {global, removed};
static uint32_t now(void) {
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return (uint32_t)(t.tv_sec * 1000 + t.tv_nsec / 1000000);
}
int main(void) {
    struct wl_display *display = wl_display_connect(NULL);
    if (!display) return 1;
    struct wl_registry *registry = wl_display_get_registry(display);
    wl_registry_add_listener(registry, &listener, NULL);
    if (wl_display_roundtrip(display) < 0 || !manager) return 2;
    struct zwlr_virtual_pointer_v1 *pointer = zwlr_virtual_pointer_manager_v1_create_virtual_pointer(manager, NULL);
    if (wl_display_roundtrip(display) < 0) return 3;
    puts("ready"); fflush(stdout);
    char line[128];
    while (fgets(line, sizeof(line), stdin)) {
        unsigned x, y;
        if (sscanf(line, "move %u %u", &x, &y) == 2 && x < 1280 && y < 720) {
            zwlr_virtual_pointer_v1_motion_absolute(pointer, now(), x, y, 1280, 720);
            zwlr_virtual_pointer_v1_frame(pointer);
        } else if (!strcmp(line, "click\n")) {
            zwlr_virtual_pointer_v1_button(pointer, now(), 0x110, WL_POINTER_BUTTON_STATE_PRESSED);
            zwlr_virtual_pointer_v1_frame(pointer);
            zwlr_virtual_pointer_v1_button(pointer, now(), 0x110, WL_POINTER_BUTTON_STATE_RELEASED);
            zwlr_virtual_pointer_v1_frame(pointer);
        } else if (!strcmp(line, "quit\n")) {
            break;
        } else { return 4; }
        if (wl_display_roundtrip(display) < 0) return 5;
        puts("ok"); fflush(stdout);
    }
    zwlr_virtual_pointer_v1_destroy(pointer);
    zwlr_virtual_pointer_manager_v1_destroy(manager);
    wl_registry_destroy(registry);
    wl_display_flush(display);
    wl_display_disconnect(display);
    return 0;
}
