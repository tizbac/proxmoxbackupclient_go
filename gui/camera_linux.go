//go:build linux && cgo && !service

package main

// The key import dialog reads a paper key's QR code with the computer's
// camera (getUserMedia). On Linux the app's window is a WebKitGTK webview,
// which refuses every camera request unless the application allows it: Wails
// does not handle WebKit's permission-request signal, and WebKit denies what
// nobody answers. prepareCamera connects a handler to the app's webview that
// allows video only (never the microphone), and only while allowCamera has
// it on (while the import dialog is open, see PrepareCamera). It also switches
// media streams on, which WebKitGTK 2.50 already has on by default.
//
// PBS_TEST_MOCK_CAMERA=1 gives the webview WebKit's built-in test camera, for
// testing on a machine without one.

/*
#cgo linux pkg-config: gtk+-3.0
#cgo !webkit2_41 pkg-config: webkit2gtk-4.0
#cgo webkit2_41 pkg-config: webkit2gtk-4.1

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

static gint pbs_cam_allow = 0;
static GMutex pbs_cam_mu;
static GCond pbs_cam_cond;
static gboolean pbs_cam_done = FALSE;
static int pbs_cam_found = 0;
static int pbs_cam_mock = 0;

static gboolean pbs_cam_on_permission(WebKitWebView *wv, WebKitPermissionRequest *req, gpointer data) {
	if (!WEBKIT_IS_USER_MEDIA_PERMISSION_REQUEST(req)) {
		return FALSE; // not ours: WebKit's default answer
	}
	WebKitUserMediaPermissionRequest *m = WEBKIT_USER_MEDIA_PERMISSION_REQUEST(req);
	if (g_atomic_int_get(&pbs_cam_allow) &&
		webkit_user_media_permission_is_for_video_device(m) &&
		!webkit_user_media_permission_is_for_audio_device(m)) {
		webkit_permission_request_allow(req);
	} else {
		webkit_permission_request_deny(req);
	}
	return TRUE;
}

static void pbs_cam_find(GtkWidget *w, gpointer data) {
	if (WEBKIT_IS_WEB_VIEW(w)) {
		WebKitWebView *wv = WEBKIT_WEB_VIEW(w);
		if (!g_object_get_data(G_OBJECT(wv), "pbs-camera")) {
			g_object_set_data(G_OBJECT(wv), "pbs-camera", GINT_TO_POINTER(1));
			g_signal_connect(wv, "permission-request", G_CALLBACK(pbs_cam_on_permission), NULL);
		}
		WebKitSettings *st = webkit_web_view_get_settings(wv);
		webkit_settings_set_enable_media_stream(st, TRUE);
		if (pbs_cam_mock) {
			webkit_settings_set_enable_mock_capture_devices(st, TRUE);
		}
		(*(int *)data)++;
		return;
	}
	if (GTK_IS_CONTAINER(w)) {
		gtk_container_forall(GTK_CONTAINER(w), pbs_cam_find, data);
	}
}

// Runs on the GTK main thread.
static gboolean pbs_cam_setup_main(gpointer unused) {
	int found = 0;
	GList *tops = gtk_window_list_toplevels();
	for (GList *l = tops; l != NULL; l = l->next) {
		pbs_cam_find(GTK_WIDGET(l->data), &found);
	}
	g_list_free(tops);
	g_mutex_lock(&pbs_cam_mu);
	pbs_cam_found = found;
	pbs_cam_done = TRUE;
	g_cond_signal(&pbs_cam_cond);
	g_mutex_unlock(&pbs_cam_mu);
	return G_SOURCE_REMOVE;
}

// From any thread but the main one: has the main thread prepare the app's
// webviews and waits for it. Returns how many were prepared, or -1 when the
// main thread did not get to it within 3 seconds.
static int pbs_cam_setup(int mock) {
	g_mutex_lock(&pbs_cam_mu);
	pbs_cam_mock = mock;
	pbs_cam_done = FALSE;
	g_mutex_unlock(&pbs_cam_mu);
	g_idle_add(pbs_cam_setup_main, NULL);
	gint64 end = g_get_monotonic_time() + 3 * G_TIME_SPAN_SECOND;
	g_mutex_lock(&pbs_cam_mu);
	while (!pbs_cam_done) {
		if (!g_cond_wait_until(&pbs_cam_cond, &pbs_cam_mu, end)) {
			break;
		}
	}
	int r = pbs_cam_done ? pbs_cam_found : -1;
	g_mutex_unlock(&pbs_cam_mu);
	return r;
}

static void pbs_cam_set_allowed(int on) {
	g_atomic_int_set(&pbs_cam_allow, on);
}
*/
import "C"

import (
	"fmt"
	"os"
)

func prepareCamera() error {
	mock := 0
	if os.Getenv("PBS_TEST_MOCK_CAMERA") == "1" {
		mock = 1
	}
	switch n := C.pbs_cam_setup(C.int(mock)); {
	case n < 0:
		return fmt.Errorf("the window did not answer in time")
	case n == 0:
		return fmt.Errorf("the app's web view was not found")
	}
	return nil
}

func allowCamera(on bool) {
	v := 0
	if on {
		v = 1
	}
	C.pbs_cam_set_allowed(C.int(v))
}
