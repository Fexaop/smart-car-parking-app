#include <LiquidCrystal_I2C.h>
#include <ESP32Servo.h>
#include "BluetoothSerial.h"

// ---------------- Bluetooth ----------------
BluetoothSerial SerialBT;
#define BT_DEVICE_NAME "ESP32_Parking"

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

// ---------------- Buttons (can still be used for manual control) ----------------
#define BTN1 32
#define BTN2 33
#define BTN3 34

// state tracking
bool prevOcc1 = false;
bool prevOcc2 = false;
bool prevOcc3 = false;

// gate status for LCD display
bool gate1Open = false;
bool gate2Open = false;
bool gate3Open = false;

// OTP display variables
String currentOTP = "";
int otpSpot = 0;
unsigned long otpDisplayTime = 0;
const unsigned long OTP_DISPLAY_DURATION = 30000; // Show OTP for 30 seconds

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
  
  // Initialize Bluetooth Serial
  if (SerialBT.begin(BT_DEVICE_NAME)) {
    Serial.println("BT: " BT_DEVICE_NAME);
  }

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
  lcd.print("ESP32 Parking");
  lcd.setCursor(0,1);
  lcd.print("BT: Ready");
  delay(1500);
  lcd.clear();
}

// Toggle gate servo (open/close)
void toggleGate(int gateNum) {
  bool* gateState;
  Servo* servo;
  
  // Get the gate state and servo
  switch (gateNum) {
    case 1:
      gateState = &gate1Open;
      servo = &s1;
      break;
    case 2:
      gateState = &gate2Open;
      servo = &s2;
      break;
    case 3:
      gateState = &gate3Open;
      servo = &s3;
      break;
    default:
      return;
  }
  
  // Toggle the gate
  *gateState = !(*gateState);
  
  if (*gateState) {
    // Open gate
    servo->write(90);
    Serial.print("Gate ");
    Serial.print(gateNum);
    Serial.println(" OPEN");
    if (SerialBT.hasClient()) {
      SerialBT.print("GATE:");
      SerialBT.print(gateNum);
      SerialBT.println(":OPEN");
    }
  } else {
    // Close gate
    servo->write(0);
    Serial.print("Gate ");
    Serial.print(gateNum);
    Serial.println(" CLOSED");
    if (SerialBT.hasClient()) {
      SerialBT.print("GATE:");
      SerialBT.print(gateNum);
      SerialBT.println(":CLOSED");
    }
  }
}

// Send occupancy update via Bluetooth
void sendOccupancyUpdate(int spot, bool occupied) {
  if (SerialBT.hasClient()) {
    SerialBT.print("UPDATE:");
    SerialBT.print(spot);
    SerialBT.print(":");
    SerialBT.println(occupied ? "1" : "0");
  }
}

// Handle Bluetooth commands
void handleBluetoothCommand(String cmd) {
  cmd.trim();
  
  // Check for OTP command format: "1:1234" (spot:otp)
  if (cmd.indexOf(':') > 0) {
    int colonPos = cmd.indexOf(':');
    String part1 = cmd.substring(0, colonPos);
    String part2 = cmd.substring(colonPos + 1);
    
    // Handle "OPEN:spot" command
    if (part1.equalsIgnoreCase("OPEN")) {
      int spot = part2.toInt();
      if (spot >= 1 && spot <= 3) {
        // Open the gate for this spot
        if (!gate1Open && spot == 1) toggleGate(1);
        else if (!gate2Open && spot == 2) toggleGate(2);
        else if (!gate3Open && spot == 3) toggleGate(3);
        // Clear OTP after opening
        currentOTP = "";
        otpSpot = 0;
      }
      return;
    }
    
    // Handle "spot:otp" format for displaying OTP
    int spot = part1.toInt();
    if (spot >= 1 && spot <= 3 && part2.length() == 4) {
      currentOTP = part2;
      otpSpot = spot;
      otpDisplayTime = millis();
      Serial.print("OTP for spot ");
      Serial.print(spot);
      Serial.print(": ");
      Serial.println(currentOTP);
      return;
    }
  }
  
  // Legacy commands
  String cmdLower = cmd;
  cmdLower.toLowerCase();
  
  if (cmdLower == "open 1" || cmdLower == "1" || cmdLower == "toggle 1") {
    toggleGate(1);
  } else if (cmdLower == "open 2" || cmdLower == "2" || cmdLower == "toggle 2") {
    toggleGate(2);
  } else if (cmdLower == "open 3" || cmdLower == "3" || cmdLower == "toggle 3") {
    toggleGate(3);
  } else if (cmdLower == "s" || cmdLower == "status") {
    long d1 = readUS(TRIG1, ECHO1);
    long d2 = readUS(TRIG2, ECHO2);
    long d3 = readUS(TRIG3, ECHO3);
    SerialBT.print("STATUS:");
    SerialBT.print(d1>40?"E":"O");
    SerialBT.print(":");
    SerialBT.print(d2>40?"E":"O");
    SerialBT.print(":");
    SerialBT.println(d3>40?"E":"O");
  }
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

  // Send updates only when state changes via Bluetooth
  if (occ1 != prevOcc1) {
    sendOccupancyUpdate(1, occ1);
    prevOcc1 = occ1;
  }
  if (occ2 != prevOcc2) {
    sendOccupancyUpdate(2, occ2);
    prevOcc2 = occ2;
  }
  if (occ3 != prevOcc3) {
    sendOccupancyUpdate(3, occ3);
    prevOcc3 = occ3;
  }

  // -------- Bluetooth Serial Commands --------
  if (SerialBT.available()) {
    String btCmd = SerialBT.readStringUntil('\n');
    btCmd.trim();
    handleBluetoothCommand(btCmd);
  }

  // -------- Button handling (manual toggle) --------
  static int lastBtn1 = HIGH;
  static int lastBtn2 = HIGH;
  static int lastBtn3 = HIGH;
  int b1 = digitalRead(BTN1);
  int b2 = digitalRead(BTN2);
  int b3 = digitalRead(BTN3);
  if (b1 == LOW && lastBtn1 == HIGH) {
    toggleGate(1);
  }
  if (b2 == LOW && lastBtn2 == HIGH) {
    toggleGate(2);
  }
  if (b3 == LOW && lastBtn3 == HIGH) {
    toggleGate(3);
  }
  lastBtn1 = b1; lastBtn2 = b2; lastBtn3 = b3;

  // -------- LCD Update showing gate status or OTP --------
  if (millis() - lastLCD > 200) {
    lastLCD = millis();

    // Check if OTP should be displayed
    bool displayOTP = false;
    if (currentOTP != "" && otpSpot > 0) {
      if (millis() - otpDisplayTime < OTP_DISPLAY_DURATION) {
        displayOTP = true;
      } else {
        // OTP expired, clear it
        currentOTP = "";
        otpSpot = 0;
      }
    }

    if (displayOTP) {
      // Display OTP on LCD
      lcd.setCursor(0,0);
      lcd.print("Spot ");
      lcd.print(otpSpot);
      lcd.print(" OTP:    ");
      
      lcd.setCursor(0,1);
      lcd.print("Code: ");
      lcd.print(currentOTP);
      lcd.print("      ");
    } else {
      // Normal status display
      lcd.setCursor(0,0);
      // Line 1: Gate status (O=Open, C=Closed, *=Occupied)
      lcd.print("G1:");
      if (gate1Open) lcd.print("O"); 
      else if (occ1) lcd.print("*");
      else lcd.print("C");
      
      lcd.print(" G2:");
      if (gate2Open) lcd.print("O");
      else if (occ2) lcd.print("*");
      else lcd.print("C");
      
      lcd.print(" G3:");
      if (gate3Open) lcd.print("O");
      else if (occ3) lcd.print("*");
      else lcd.print("C");
      
      lcd.print(" ");

      lcd.setCursor(0,1);
      // Line 2: Distance readings
      lcd.print(d1);
      lcd.print("cm ");
      lcd.print(d2);
      lcd.print("cm ");
      lcd.print(d3);
      lcd.print("cm  ");
    }
  }

  delay(50); // small stability delay
}
