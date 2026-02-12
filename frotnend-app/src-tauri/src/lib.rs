// Learn more about Tauri commands at https://tauri.app/develop/calling-rust/
#[tauri::command]
fn greet(name: &str) -> String {
    format!("Hello, {}! You've been greeted from Rust!", name)
}

#[tauri::command]
fn is_mobile() -> bool {
    #[cfg(target_os = "android")]
    return true;
    
    #[cfg(target_os = "ios")]
    return true;
    
    #[cfg(not(any(target_os = "android", target_os = "ios")))]
    return false;
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_google_auth::init())
        .invoke_handler(tauri::generate_handler![greet, is_mobile])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
