#define MQ2 34
#define IR 32
#define BUZZER 25

void setup(){
  Serial.begin(115200);
  
  pinMode(IR, INPUT);
  pinMode(BUZZER, OUTPUT);

  digitalWrite(BUZZER, LOW); // Buzzer OFF
}

void loop(){
  int smoke = analogRead(MQ2);
  int ir = digitalRead(IR);
  
  if(smoke > 100 && ir != LOW){
    Serial.println("ALERT: smoke/fire detected!");
    digitalWrite(BUZZER, HIGH);  // Buzzer ON
  }
  else{
    Serial.println("Clear");

    digitalWrite(BUZZER, LOW);   // Buzzer OFF
  }

  Serial.print("Smoke: ");
  Serial.println(smoke);

  Serial.print("IR: ");
  Serial.println(ir);

  delay(500);
}