#include <LiquidCrystal_I2C.h>
#include <ESP32Servo.h>

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
}

void loop() {

  // -------- Read Sensors --------
  long d1 = readUS(TRIG1, ECHO1);
  long d2 = readUS(TRIG2, ECHO2);
  long d3 = readUS(TRIG3, ECHO3);

  // -------- Servo Logic --------
  s1.write(d1 < 20 ? 90 : 0);
  s2.write(d2 < 20 ? 90 : 0);
  s3.write(d3 < 20 ? 90 : 0);

  // -------- Serial Output --------
  Serial.print("D1: "); Serial.print(d1);
  Serial.print("  D2: "); Serial.print(d2);
  Serial.print("  D3: "); Serial.println(d3);

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
