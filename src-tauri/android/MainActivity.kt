package app.tuneharbor.android

import android.webkit.WebView
import androidx.activity.OnBackPressedCallback

class MainActivity : TauriActivity() {
    override val handleBackNavigation = false

    override fun onWebViewCreate(webView: WebView) {
        super.onWebViewCreate(webView)
        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                webView.evaluateJavascript("window.__MUSIC_ANDROID_BACK__ ? window.__MUSIC_ANDROID_BACK__() : false") { handled ->
                    if (handled != "true") {
                        if (webView.url?.startsWith("http://tauri.localhost/") == true) {
                            finish()
                        } else {
                            webView.loadUrl("http://tauri.localhost/android.html")
                        }
                    }
                }
            }
        })
    }
}
