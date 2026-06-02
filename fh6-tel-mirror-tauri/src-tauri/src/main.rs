#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::collections::HashMap;
use std::sync::atomic::{AtomicI64, AtomicU64, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use tauri::Manager;
use tokio::net::UdpSocket;
use tokio::sync::{Notify, RwLock};

#[derive(serde::Serialize, serde::Deserialize, Clone, Debug)]
struct Destination {
    id: String,
    name: String,
    host: String,
    port: u16,
    #[serde(rename = "rateLimit")]
    rate_limit: String, // "60Hz", "30Hz", "20Hz", "10Hz"
    enabled: bool,
}

#[derive(serde::Serialize, serde::Deserialize, Clone, Debug)]
struct AppConfig {
    #[serde(rename = "bindPort")]
    bind_port: u16,
    destinations: Vec<Destination>,
}

struct AppState {
    config: Arc<RwLock<AppConfig>>,
    rx_packets: Arc<AtomicU64>,
    tx_packets: Arc<AtomicU64>,
    total_delay: Arc<AtomicI64>,
    live_pps: Arc<AtomicU64>,
    udp_trigger: Arc<Notify>,
    target_counters: Arc<RwLock<HashMap<String, Arc<AtomicU64>>>>,
}

const CONFIG_FILE: &str = "mirror_config.json";

fn load_config() -> AppConfig {
    if let Ok(data) = std::fs::read_to_string(CONFIG_FILE) {
        if let Ok(cfg) = serde_json::from_str(&data) {
            return cfg;
        }
    }

    // Default setup
    AppConfig {
        bind_port: 20440,
        destinations: vec![
            Destination {
                id: "simhub".to_string(),
                name: "SimHub/Vibration Rig".to_string(),
                host: "127.0.0.1".to_string(),
                port: 20500,
                rate_limit: "60Hz".to_string(),
                enabled: false,
            },
            Destination {
                id: "local-hub".to_string(),
                name: "Local FH6 Telemetry Server".to_string(),
                host: "127.0.0.1".to_string(),
                port: 20450,
                rate_limit: "60Hz".to_string(),
                enabled: true,
            },
        ],
    }
}

fn save_config(cfg: &AppConfig) {
    if let Ok(data) = serde_json::to_string_pretty(cfg) {
        let _ = std::fs::write(CONFIG_FILE, data);
    }
}

async fn start_udp_listener(state: Arc<AppState>) {
    let mut buffer = [0u8; 512];
    loop {
        let bind_port = {
            let config_guard = state.config.read().await;
            config_guard.bind_port
        };

        let addr = format!("0.0.0.0:{}", bind_port);
        let socket = match UdpSocket::bind(&addr).await {
            Ok(s) => Arc::new(s),
            Err(e) => {
                eprintln!("[UDP] Bind failure on {}: {}", addr, e);
                tokio::time::sleep(Duration::from_secs(1)).await;
                continue;
            }
        };

        println!("[UDP] Listening on {}", addr);

        loop {
            tokio::select! {
                _ = state.udp_trigger.notified() => {
                    println!("[UDP] Inbound port updated. Re-binding listener...");
                    break;
                }
                res = socket.recv_from(&mut buffer) => {
                    match res {
                        Ok((n, _)) => {
                            if n > 0 {
                                state.rx_packets.fetch_add(1, Ordering::Relaxed);
                                let packet_data = buffer[..n].to_vec();
                                let state_clone = Arc::clone(&state);
                                let socket_clone = Arc::clone(&socket);
                                tokio::spawn(async move {
                                    route_packet(packet_data, state_clone, socket_clone).await;
                                });
                            }
                        }
                        Err(_) => {
                            break;
                        }
                    }
                }
            }
        }
    }
}

async fn route_packet(data: Vec<u8>, state: Arc<AppState>, socket: Arc<UdpSocket>) {
    let start = Instant::now();
    let config_guard = state.config.read().await;
    let counters_guard = state.target_counters.read().await;

    for dest in &config_guard.destinations {
        if !dest.enabled {
            continue;
        }

        let counter = if let Some(c) = counters_guard.get(&dest.id) {
            Arc::clone(c)
        } else {
            continue;
        };

        let count = counter.fetch_add(1, Ordering::Relaxed);

        let skip = match dest.rate_limit.as_str() {
            "30Hz" => count % 2 != 0,
            "20Hz" => count % 3 != 0,
            "10Hz" => count % 6 != 0,
            _ => false,
        };

        if skip {
            continue;
        }

        let addr = format!("{}:{}", dest.host, dest.port);
        let socket_clone = Arc::clone(&socket);
        let tx_packets_clone = Arc::clone(&state.tx_packets);
        let data_clone = data.clone();

        tokio::spawn(async move {
            if let Ok(sent) = socket_clone.send_to(&data_clone, &addr).await {
                if sent > 0 {
                    tx_packets_clone.fetch_add(1, Ordering::Relaxed);
                }
            }
        });
    }

    let delay = start.elapsed().as_nanos() as i64;
    state.total_delay.fetch_add(delay, Ordering::Relaxed);
}

async fn sync_counters(state: &AppState) {
    let config_guard = state.config.read().await;
    let mut counters_guard = state.target_counters.write().await;

    counters_guard.retain(|id, _| config_guard.destinations.iter().any(|d| &d.id == id));

    for dest in &config_guard.destinations {
        if !counters_guard.contains_key(&dest.id) {
            counters_guard.insert(dest.id.clone(), Arc::new(AtomicU64::new(0)));
        }
    }
}

// ----- TAURI COMMANDS -----

#[tauri::command]
async fn get_config(state: tauri::State<'_, Arc<AppState>>) -> Result<AppConfig, String> {
    let config_guard = state.config.read().await;
    Ok(config_guard.clone())
}

#[tauri::command]
async fn update_bind_port(
    port: u16,
    state: tauri::State<'_, Arc<AppState>>,
) -> Result<bool, String> {
    if port == 0 {
        return Err("Invalid port".to_string());
    }

    let mut config_guard = state.config.write().await;
    let changed = config_guard.bind_port != port;
    config_guard.bind_port = port;
    save_config(&config_guard);

    if changed {
        state.udp_trigger.notify_one();
    }

    Ok(true)
}

#[tauri::command]
async fn add_destination(
    name: String,
    host: String,
    port: u16,
    rate_limit: String,
    state: tauri::State<'_, Arc<AppState>>,
) -> Result<bool, String> {
    if name.is_empty() || host.is_empty() || port == 0 {
        return Err("Invalid arguments".to_string());
    }

    let id = format!(
        "node-{}",
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs()
    );

    let mut config_guard = state.config.write().await;
    config_guard.destinations.push(Destination {
        id: id.clone(),
        name,
        host,
        port,
        rate_limit,
        enabled: true,
    });
    save_config(&config_guard);
    drop(config_guard);

    sync_counters(&state).await;
    Ok(true)
}

#[tauri::command]
async fn toggle_destination(
    id: String,
    enabled: bool,
    state: tauri::State<'_, Arc<AppState>>,
) -> Result<bool, String> {
    let mut config_guard = state.config.write().await;
    for dest in &mut config_guard.destinations {
        if dest.id == id {
            dest.enabled = enabled;
            save_config(&config_guard);
            return Ok(true);
        }
    }
    Err("Node not found".to_string())
}

#[tauri::command]
async fn delete_destination(
    id: String,
    state: tauri::State<'_, Arc<AppState>>,
) -> Result<bool, String> {
    let mut config_guard = state.config.write().await;
    let len_before = config_guard.destinations.len();
    config_guard.destinations.retain(|d| d.id != id);
    let changed = config_guard.destinations.len() != len_before;

    if changed {
        save_config(&config_guard);
        drop(config_guard);
        sync_counters(&state).await;
        Ok(true)
    } else {
        Err("Node not found".to_string())
    }
}

fn main() {
    let initial_config = load_config();

    let state = Arc::new(AppState {
        config: Arc::new(RwLock::new(initial_config)),
        rx_packets: Arc::new(AtomicU64::new(0)),
        tx_packets: Arc::new(AtomicU64::new(0)),
        total_delay: Arc::new(AtomicI64::new(0)),
        live_pps: Arc::new(AtomicU64::new(0)),
        udp_trigger: Arc::new(Notify::new()),
        target_counters: Arc::new(RwLock::new(HashMap::new())),
    });

    tauri::Builder::default()
        .manage(Arc::clone(&state))
        .setup(move |app| {
            let state_ref = Arc::clone(&state);
            let app_handle = app.handle();

            // All async tasks are spawned inside Tauri's Tokio runtime (setup hook)
            tauri::async_runtime::spawn(async move {
                // 1. Sync counters on startup
                sync_counters(&state_ref).await;

                // 2. Start the UDP listener loop
                let state_listener = Arc::clone(&state_ref);
                tauri::async_runtime::spawn(async move {
                    start_udp_listener(state_listener).await;
                });

                // 3. PPS stats ticker
                let rx_clone = Arc::clone(&state_ref.rx_packets);
                let pps_clone = Arc::clone(&state_ref.live_pps);
                tauri::async_runtime::spawn(async move {
                    let mut prev_rx: u64 = 0;
                    loop {
                        tokio::time::sleep(Duration::from_secs(1)).await;
                        let current_rx = rx_clone.load(Ordering::Relaxed);
                        let pps = current_rx.saturating_sub(prev_rx);
                        prev_rx = current_rx;
                        pps_clone.store(pps, Ordering::Relaxed);
                    }
                });

                // 4. Stats emitter loop → sends real-time metrics to frontend
                let rx_e = Arc::clone(&state_ref.rx_packets);
                let tx_e = Arc::clone(&state_ref.tx_packets);
                let pps_e = Arc::clone(&state_ref.live_pps);
                let delay_e = Arc::clone(&state_ref.total_delay);
                tauri::async_runtime::spawn(async move {
                    loop {
                        tokio::time::sleep(Duration::from_secs(1)).await;
                        let rx = rx_e.load(Ordering::Relaxed);
                        let tx = tx_e.load(Ordering::Relaxed);
                        let pps = pps_e.load(Ordering::Relaxed);
                        let avg_us = if rx > 0 {
                            (delay_e.load(Ordering::Relaxed) / rx as i64) / 1000
                        } else {
                            0
                        };

                        let _ = app_handle.emit_all(
                            "stats_update",
                            serde_json::json!({
                                "rxPackets": rx,
                                "txPackets": tx,
                                "pps": pps,
                                "avgUs": avg_us
                            }),
                        );
                    }
                });
            });

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            get_config,
            update_bind_port,
            add_destination,
            toggle_destination,
            delete_destination
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
