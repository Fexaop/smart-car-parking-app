#include <LiquidCrystal_I2C.h>
#include <ESP32Servo.h>
#include <WiFi.h>
#include <HTTPClient.h>

// ---------------- LCD ----------------
LiquidCrystal_I2C lcd(0x27, 16, 2);

// ---------------- Ultrasonic Pins ----------------
#define TRIG1 5
#define ECHO1 18
#define TRIG2 17
#define ECHO2 16
#define TRIG3 4
#define ECHO3 2

// ---------------- Servo Pins ----------------
Servo s1, s2, s3;
#define SERVO1 25
#define SERVO2 26
#define SERVO3 27

// ---------------- WiFi / Server ----------------
// TODO: set these to your network and backend server IP
#define WIFI_SSID "Gunit’s iPhone"
#define WIFI_PASS "1234567890"
#define SERVER_HOST "192.168.1.100" // backend machine IP
#define SERVER_PORT 8080

// ---------------- Buttons ----------------
#define BTN1 32
#define BTN2 33
#define BTN3 34

// state tracking to avoid spamming server
bool prevOcc1 = false;
bool prevOcc2 = false;
bool prevOcc3 = false;

// ---------------- Timing ----------------
unsigned long lastLCD = 0;

// ---------------- Ultrasonic Function ----------------
long readUS(int trig, int echo) {
  digitalWrite(trig, LOW);
  delayMicroseconds(2);

  digitalWrite(trig, HIGH);
  delayMicroseconds(10);
  digitalWrite(trig, LOW);

  long dur = pulseIn(echo, HIGH, 30000); // 30ms timeout
  long dist = dur * 0.034 / 2;
  if (dist == 0) dist = 999; // no echo fallback
  return dist;
}

void setup() {
  Serial.begin(115200);

  // Ultrasonic pin modes
  pinMode(TRIG1, OUTPUT); pinMode(ECHO1, INPUT);
  pinMode(TRIG2, OUTPUT); pinMode(ECHO2, INPUT);
  pinMode(TRIG3, OUTPUT); pinMode(ECHO3, INPUT);

  // Buttons
  pinMode(BTN1, INPUT_PULLUP);
  pinMode(BTN2, INPUT_PULLUP);
  pinMode(BTN3, INPUT_PULLUP);

  // Servo attach
  s1.attach(SERVO1);
  s2.attach(SERVO2);
  s3.attach(SERVO3);

  // LCD start
  lcd.init();
  lcd.backlight();
  lcd.setCursor(0,0);
  lcd.print("System Start");
  delay(1200);
  lcd.clear();

  // connect WiFi
  connectWiFi();
}

void connectWiFi() {
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASS);
  Serial.print("Connecting to WiFi");
  unsigned long start = millis();
  while (WiFi.status() != WL_CONNECTED && millis() - start < 20000) {
    Serial.print('.');
    delay(500);
  }
  if (WiFi.status() == WL_CONNECTED) {
    Serial.println();
    Serial.print("WiFi connected, IP: ");
    Serial.println(WiFi.localIP());
  } else {
    Serial.println();
    Serial.println("WiFi connect failed");
  }
}

String serverUrl() {
  return String("http://") + SERVER_HOST + ":" + String(SERVER_PORT);
}

void sendUpdate(const char* parkID, const char* spotID, bool occupied) {
  if (WiFi.status() != WL_CONNECTED) return;
  HTTPClient http;
  String url = serverUrl() + "/api/parks/" + parkID + "/spots/" + spotID + "/update";
  http.begin(url);
  http.addHeader("Content-Type", "application/json");
  String body = String("{\"occupied\":") + (occupied ? "true" : "false") + String("}");
  int code = http.POST(body);
  if (code > 0) {
    Serial.print("Update " ); Serial.print(spotID); Serial.print(" -> "); Serial.println(code);
  } else {
    Serial.print("Update failed: "); Serial.println(http.errorToString(code));
  }
  http.end();
}

void sendOpenGate(const char* parkID, const char* spotID) {
  if (WiFi.status() != WL_CONNECTED) return;
  HTTPClient http;
  String url = serverUrl() + "/api/parks/" + parkID + "/spots/" + spotID + "/opengate";
  http.begin(url);
  http.addHeader("Content-Type", "application/json");
  int code = http.POST("{}");
  if (code > 0) {
    Serial.print("OpenGate " ); Serial.print(spotID); Serial.print(" -> "); Serial.println(code);
  } else {
    Serial.print("OpenGate failed: "); Serial.println(http.errorToString(code));
  }
  http.end();
}

void loop() {

  // -------- Read Sensors --------
  long d1 = readUS(TRIG1, ECHO1);
  long d2 = readUS(TRIG2, ECHO2);
  long d3 = readUS(TRIG3, ECHO3);

  // -------- Determine occupancy (empty if distance > 40cm) --------
  const int EMPTY_THRESHOLD = 40;
  bool occ1 = !(d1 > EMPTY_THRESHOLD);
  bool occ2 = !(d2 > EMPTY_THRESHOLD);
  bool occ3 = !(d3 > EMPTY_THRESHOLD);

  // Send updates only when state changes
  if (occ1 != prevOcc1) {
    sendUpdate("park-1", "park-1-spot-1", occ1);
    prevOcc1 = occ1;
  }
  if (occ2 != prevOcc2) {
    sendUpdate("park-1", "park-1-spot-2", occ2);
    prevOcc2 = occ2;
  }
  if (occ3 != prevOcc3) {
    sendUpdate("park-1", "park-1-spot-3", occ3);
    prevOcc3 = occ3;
  }

  // -------- Button handling (open gate per spot) --------
  static int lastBtn1 = HIGH;
  static int lastBtn2 = HIGH;
  static int lastBtn3 = HIGH;
  int b1 = digitalRead(BTN1);
  int b2 = digitalRead(BTN2);
  int b3 = digitalRead(BTN3);
  if (b1 == LOW && lastBtn1 == HIGH) {
    // pressed
    s1.write(90);
    sendOpenGate("park-1", "park-1-spot-1");
    delay(1500);
    s1.write(0);
  }
  if (b2 == LOW && lastBtn2 == HIGH) {
    s2.write(90);
    sendOpenGate("park-1", "park-1-spot-2");
    delay(1500);
    s2.write(0);
  }
  if (b3 == LOW && lastBtn3 == HIGH) {
    s3.write(90);
    sendOpenGate("park-1", "park-1-spot-3");
    delay(1500);
    s3.write(0);
  }
  lastBtn1 = b1; lastBtn2 = b2; lastBtn3 = b3;

  // -------- Serial TUI + Output --------
  static unsigned long lastPrint = 0;
  if (millis() - lastPrint > 1000) {
    lastPrint = millis();
    Serial.println("===== Parking TUI =====");
    Serial.print("Slot1: "); Serial.print(occ1 ? "OCCUPIED" : "EMPTY"); Serial.print("  "); Serial.print(d1); Serial.println(" cm");
    Serial.print("Slot2: "); Serial.print(occ2 ? "OCCUPIED" : "EMPTY"); Serial.print("  "); Serial.print(d2); Serial.println(" cm");
    Serial.print("Slot3: "); Serial.print(occ3 ? "OCCUPIED" : "EMPTY"); Serial.print("  "); Serial.print(d3); Serial.println(" cm");
    Serial.println("Press BTN1/BTN2/BTN3 to open corresponding gate.");
  }

  // -------- LCD Update (Once per 1 sec) --------
  if (millis() - lastLCD > 1000) {
    lastLCD = millis();

    lcd.setCursor(0,0);
    lcd.print("D1:");
    lcd.print(d1);
    lcd.print(" D2:");
    lcd.print(d2);
    lcd.print("   ");   // clear leftovers

    lcd.setCursor(0,1);
    lcd.print("D3:");
    lcd.print(d3);
    lcd.print(" cm   "); // clear leftovers
  }

  delay(50); // small stability delay
}
