#ifndef KH_SPARKLE_H
#define KH_SPARKLE_H

#ifdef __cplusplus
extern "C" {
#endif

/* 0 idle, 1 updating, 2 relaunching. */
typedef void (*kh_sparkle_state_fn)(int state);

/* Starts the updater. Config comes from the bundle's Info.plist
 * (SUFeedURL, SUPublicEDKey, KHHoldURL). Returns 0 when started; non-zero
 * (with kh_sparkle_last_error() set) when refused. */
int kh_sparkle_start(void);

/* Last refusal/failure reason, for a log line. Valid until the next call from
 * another thread; copy it if you keep it. Never NULL. */
const char *kh_sparkle_last_error(void);

void kh_sparkle_set_state_callback(kh_sparkle_state_fn cb);

int kh_sparkle_state(void);

#ifdef __cplusplus
}
#endif

#endif
