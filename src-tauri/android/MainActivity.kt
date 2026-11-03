package app.tuneharbor.android

import android.webkit.WebView
import android.view.ViewGroup
import androidx.activity.OnBackPressedCallback
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat

class MainActivity : TauriActivity() {
    override val handleBackNavigation = false

    override fun onWebViewCreate(webView: WebView) {
        super.onWebViewCreate(webView)
        window.decorView.setBackgroundColor(android.graphics.Color.BLACK)
        ViewCompat.setOnApplyWindowInsetsListener(webView) { view, insets ->
            val safe = insets.getInsets(WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.displayCutout() or WindowInsetsCompat.Type.ime())
            (view.layoutParams as? ViewGroup.MarginLayoutParams)?.let { params ->
                params.setMargins(safe.left, safe.top, safe.right, safe.bottom)
                view.layoutParams = params
            }
            WindowInsetsCompat.CONSUMED
        }
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
