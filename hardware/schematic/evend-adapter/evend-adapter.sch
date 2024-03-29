EESchema Schematic File Version 4
LIBS:evend-adapter-cache
EELAYER 29 0
EELAYER END
$Descr A4 11693 8268
encoding utf-8
Sheet 1 1
Title ""
Date ""
Rev ""
Comp ""
Comment1 ""
Comment2 ""
Comment3 ""
Comment4 ""
$EndDescr
$Comp
L evend-adapter-rescue:BC807-transistors Q2
U 1 1 5B08F43C
P 10100 1200
F 0 "Q2" H 10300 1275 50  0000 L CNN
F 1 "BC807" H 10300 1200 50  0000 L CNN
F 2 "evend-adapter:SOT-23" H 10300 1125 50  0001 L CIN
F 3 "" H 10100 1200 50  0001 L CNN
	1    10100 1200
	-1   0    0    1   
$EndComp
$Comp
L evend-adapter-rescue:BC807-transistors Q3
U 1 1 5B08F4F9
P 10250 1450
F 0 "Q3" H 10450 1525 50  0000 L CNN
F 1 "BC807" H 10450 1450 50  0000 L CNN
F 2 "evend-adapter:SOT-23" H 10450 1375 50  0001 L CIN
F 3 "" H 10250 1450 50  0001 L CNN
	1    10250 1450
	1    0    0    1   
$EndComp
$Comp
L Device:R R3
U 1 1 5B08F67D
P 10350 1000
F 0 "R3" V 10430 1000 50  0000 C CNN
F 1 "5R6" V 10350 1000 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 10280 1000 50  0001 C CNN
F 3 "" H 10350 1000 50  0001 C CNN
	1    10350 1000
	1    0    0    -1  
$EndComp
$Comp
L Device:R R4
U 1 1 5B08F7FA
P 10350 2250
F 0 "R4" V 10430 2250 50  0000 C CNN
F 1 "680R" V 10350 2250 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 10280 2250 50  0001 C CNN
F 3 "" H 10350 2250 50  0001 C CNN
	1    10350 2250
	1    0    0    -1  
$EndComp
Text GLabel 9650 1450 0    60   Input ~ 0
TTL-TX
Text GLabel 10500 2050 2    60   Input ~ 0
MDB-MT
Text GLabel 9300 2750 0    60   Input ~ 0
TTL-RX
Text GLabel 9700 2750 2    60   Input ~ 0
MDB-MR
Wire Wire Line
	10350 1150 10350 1200
Wire Wire Line
	10300 1200 10350 1200
Connection ~ 10350 1200
Wire Wire Line
	10050 1450 10000 1450
Wire Wire Line
	10000 1400 10000 1450
Connection ~ 10000 1450
Wire Wire Line
	10350 2050 10500 2050
Connection ~ 10350 2050
Text GLabel 1450 2950 0    60   Input ~ 0
TTL-RX
Wire Wire Line
	10350 850  10000 850 
Wire Wire Line
	10000 800  10000 850 
Wire Wire Line
	10350 2400 10350 2500
Connection ~ 10000 850 
$Comp
L power:GND #PWR0107
U 1 1 5B09563E
P 4200 1450
F 0 "#PWR0107" H 4200 1200 50  0001 C CNN
F 1 "GND" H 4200 1300 50  0000 C CNN
F 2 "" H 4200 1450 50  0001 C CNN
F 3 "" H 4200 1450 50  0001 C CNN
	1    4200 1450
	1    0    0    -1  
$EndComp
$Comp
L power:GND #PWR0108
U 1 1 5B095685
P 5200 1450
F 0 "#PWR0108" H 5200 1200 50  0001 C CNN
F 1 "GND" H 5200 1300 50  0000 C CNN
F 2 "" H 5200 1450 50  0001 C CNN
F 3 "" H 5200 1450 50  0001 C CNN
	1    5200 1450
	1    0    0    -1  
$EndComp
Wire Wire Line
	10350 1200 10350 1250
Wire Wire Line
	10000 1450 9950 1450
Wire Wire Line
	10350 2050 10350 2100
Wire Wire Line
	10000 850  10000 1000
$Comp
L Display_Character:RC1602A U1
U 1 1 5CD310D3
P 1950 6200
F 0 "U1" H 1750 6850 50  0000 C CNN
F 1 "RC1602A" H 2150 6850 50  0000 C CNN
F 2 "evend-adapter:LCD-016N002L" H 2050 5400 50  0000 C CNN
F 3 "http://www.raystar-optronics.com/down.php?ProID=18" H 2050 6100 50  0001 C CNN
	1    1950 6200
	1    0    0    -1  
$EndComp
Text GLabel 1450 2850 0    60   Input ~ 0
TTL-TX
$Comp
L power:GND #PWR0114
U 1 1 5CD395F6
P 2050 4500
F 0 "#PWR0114" H 2050 4250 50  0001 C CNN
F 1 "GND" V 2050 4300 50  0000 C CNN
F 2 "" H 2050 4500 50  0001 C CNN
F 3 "" H 2050 4500 50  0001 C CNN
	1    2050 4500
	1    0    0    -1  
$EndComp
Text GLabel 3400 3300 0    60   Input ~ 0
SCL
$Comp
L power:GND #PWR0116
U 1 1 5CD71E01
P 3800 3450
F 0 "#PWR0116" H 3800 3200 50  0001 C CNN
F 1 "GND" H 3800 3300 50  0000 C CNN
F 2 "" H 3800 3450 50  0001 C CNN
F 3 "" H 3800 3450 50  0001 C CNN
	1    3800 3450
	1    0    0    -1  
$EndComp
Text GLabel 3400 3200 0    60   Input ~ 0
SDA
$Comp
L Device:R R8
U 1 1 5CD7B3C2
P 3700 2950
F 0 "R8" V 3780 2950 50  0000 C CNN
F 1 "4K7" V 3700 2950 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 3630 2950 50  0001 C CNN
F 3 "" H 3700 2950 50  0001 C CNN
	1    3700 2950
	1    0    0    -1  
$EndComp
$Comp
L Device:R R9
U 1 1 5CD7C818
P 3500 2950
F 0 "R9" V 3580 2950 50  0000 C CNN
F 1 "4K7" V 3500 2950 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 3430 2950 50  0001 C CNN
F 3 "" H 3500 2950 50  0001 C CNN
	1    3500 2950
	1    0    0    -1  
$EndComp
Wire Wire Line
	3900 3200 3500 3200
Wire Wire Line
	3400 3300 3700 3300
Wire Wire Line
	3500 3100 3500 3200
Connection ~ 3500 3200
Wire Wire Line
	3500 3200 3400 3200
Wire Wire Line
	3700 3100 3700 3300
Connection ~ 3700 3300
Wire Wire Line
	3700 3300 3900 3300
Wire Wire Line
	3900 3000 3850 3000
Wire Wire Line
	3700 2800 3700 2750
Wire Wire Line
	3500 2800 3500 2750
Wire Wire Line
	3900 3100 3800 3100
Wire Wire Line
	3800 3100 3800 3450
Text GLabel 2550 3150 2    60   Input ~ 0
SCL
Text GLabel 2550 3350 2    60   Input ~ 0
SDA
Text GLabel 3350 3950 2    60   Input ~ 0
MOSI-5V
Text GLabel 2550 3850 2    60   Input ~ 0
MISO-5V
Text GLabel 3600 5050 0    60   Input ~ 0
MOSI-3V
Text GLabel 3600 5150 0    60   Input ~ 0
MISO-3V
$Comp
L power:GND #PWR0118
U 1 1 5CD87503
P 3600 5350
F 0 "#PWR0118" H 3600 5100 50  0001 C CNN
F 1 "GND" V 3600 5150 50  0000 C CNN
F 2 "" H 3600 5350 50  0001 C CNN
F 3 "" H 3600 5350 50  0001 C CNN
	1    3600 5350
	0    1    1    0   
$EndComp
$Comp
L Connector_Generic:Conn_02x03_Top_Bottom J3
U 1 1 5CD8CD77
P 1750 1000
F 0 "J3" H 1800 1225 50  0000 C CNN
F 1 "Conn_02x03_Top_Bottom" H 1800 1226 50  0001 C CNN
F 2 "evend-adapter:Molex_MiniFit-JR-5569-06A2_2x03x4.20mm_Angled" H 1750 1000 50  0001 C CNN
F 3 "~" H 1750 1000 50  0001 C CNN
	1    1750 1000
	1    0    0    -1  
$EndComp
Text GLabel 2050 1000 2    60   Input ~ 0
MDB-MT
Text GLabel 2050 900  2    60   Input ~ 0
MDB-MR
NoConn ~ 1550 1100
$Comp
L power:GND #PWR0104
U 1 1 5CD90EFC
P 2050 1100
F 0 "#PWR0104" H 2050 850 50  0001 C CNN
F 1 "GND" H 2050 950 50  0000 C CNN
F 2 "" H 2050 1100 50  0001 C CNN
F 3 "" H 2050 1100 50  0001 C CNN
	1    2050 1100
	1    0    0    -1  
$EndComp
$Comp
L power:GND #PWR0105
U 1 1 5CD9179A
P 1200 1200
F 0 "#PWR0105" H 1200 950 50  0001 C CNN
F 1 "GND" H 1200 1050 50  0000 C CNN
F 2 "" H 1200 1200 50  0001 C CNN
F 3 "" H 1200 1200 50  0001 C CNN
	1    1200 1200
	1    0    0    -1  
$EndComp
NoConn ~ 1550 6000
NoConn ~ 1550 6100
NoConn ~ 1550 6200
NoConn ~ 1550 6300
NoConn ~ 3600 4150
NoConn ~ 3600 4250
NoConn ~ 3600 4350
NoConn ~ 3600 4650
NoConn ~ 3600 4750
NoConn ~ 3600 4850
NoConn ~ 3600 4950
NoConn ~ 3600 6050
NoConn ~ 3600 5450
NoConn ~ 5850 5750
NoConn ~ 5850 5950
NoConn ~ 5850 4750
NoConn ~ 3600 5550
NoConn ~ 5850 5050
NoConn ~ 5850 5850
NoConn ~ 5850 5450
NoConn ~ 5850 5650
$Comp
L evend-adapter-rescue:dc24-dc5-module psu1
U 1 1 5CE02DDD
P 4700 1250
F 0 "psu1" H 4700 1717 50  0000 C CNN
F 1 "dc24-dc5-module" H 4700 1626 50  0000 C CNN
F 2 "evend-adapter:PSU_24-5_копия" H 4700 1650 50  0001 C CNN
F 3 "" H 4700 1650 50  0001 C CNN
	1    4700 1250
	1    0    0    -1  
$EndComp
Text GLabel 1250 900  0    60   Input ~ 0
MDB+24V
Text GLabel 4200 1050 0    60   Input ~ 0
MDB+24V
Text GLabel 2550 3750 2    60   Input ~ 0
SCK-5V
Text GLabel 2550 4050 2    60   Input ~ 0
SS-5V
Text GLabel 3600 4450 0    60   Input ~ 0
INT-3V
$Comp
L power:GND #PWR0101
U 1 1 5CE07FFA
P 1950 6900
F 0 "#PWR0101" H 1950 6650 50  0001 C CNN
F 1 "GND" H 1950 6750 50  0000 C CNN
F 2 "" H 1950 6900 50  0001 C CNN
F 3 "" H 1950 6900 50  0001 C CNN
	1    1950 6900
	1    0    0    -1  
$EndComp
Text GLabel 1550 5700 0    60   Input ~ 0
A0
Text GLabel 1550 5800 0    60   Input ~ 0
RW
Text GLabel 1550 5900 0    60   Input ~ 0
E
Text GLabel 1550 6400 0    60   Input ~ 0
D4
Text GLabel 1550 6500 0    60   Input ~ 0
D5
Text GLabel 1550 6600 0    60   Input ~ 0
D6
Text GLabel 1550 6700 0    60   Input ~ 0
D7
Text GLabel 5850 4450 2    60   Input ~ 0
A0
Text GLabel 5850 4550 2    60   Input ~ 0
RW
Text GLabel 5850 4650 2    60   Input ~ 0
E
Text GLabel 5850 4850 2    60   Input ~ 0
D4
Text GLabel 5850 4950 2    60   Input ~ 0
D5
Text GLabel 5850 5150 2    60   Input ~ 0
D6
Text GLabel 5850 5350 2    60   Input ~ 0
D7
Wire Wire Line
	9650 2750 9700 2750
Wire Wire Line
	9650 2700 9650 2750
Connection ~ 9650 2750
Wire Wire Line
	9600 2750 9650 2750
NoConn ~ 5850 6050
$Comp
L Connector:Conn_01x04_Male J2
U 1 1 5CD701D9
P 4100 3200
F 0 "J2" H 4208 3481 50  0000 C CNN
F 1 "Keyboard" H 4208 3390 50  0000 C CNN
F 2 "evend-adapter:JST_PH_B4B-PH-K_04x2.00mm_Straight" H 4100 3200 50  0001 C CNN
F 3 "~" H 4100 3200 50  0001 C CNN
	1    4100 3200
	-1   0    0    1   
$EndComp
$Comp
L Transistor_FET:2N7002 Q4
U 1 1 5CD93225
P 7300 1250
F 0 "Q4" V 7551 1250 50  0000 C CNN
F 1 "2N7002" V 7642 1250 50  0000 C CNN
F 2 "evend-adapter:SOT-23" H 7500 1175 50  0001 L CIN
F 3 "https://www.fairchildsemi.com/datasheets/2N/2N7002.pdf" H 7300 1250 50  0001 L CNN
	1    7300 1250
	0    1    1    0   
$EndComp
$Comp
L Device:R R11
U 1 1 5CD9834F
P 7450 950
F 0 "R11" V 7530 950 50  0000 C CNN
F 1 "4k7" V 7450 950 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7380 950 50  0001 C CNN
F 3 "" H 7450 950 50  0001 C CNN
	1    7450 950 
	0    -1   -1   0   
$EndComp
$Comp
L Device:R R10
U 1 1 5CD98B31
P 7100 950
F 0 "R10" V 7180 950 50  0000 C CNN
F 1 "10k" V 7100 950 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7030 950 50  0001 C CNN
F 3 "" H 7100 950 50  0001 C CNN
	1    7100 950 
	0    1    1    0   
$EndComp
$Comp
L Device:R R22
U 1 1 5CD9E593
P 7700 1150
F 0 "R22" V 7780 1150 50  0000 C CNN
F 1 "10k" V 7700 1150 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7630 1150 50  0001 C CNN
F 3 "" H 7700 1150 50  0001 C CNN
	1    7700 1150
	1    0    0    -1  
$EndComp
Wire Wire Line
	7250 950  7300 950 
Wire Wire Line
	7300 1050 7300 950 
Text GLabel 7100 1350 0    60   Input ~ 0
MOSI-3V
Text GLabel 7750 1350 2    60   Input ~ 0
MOSI-5V
Wire Wire Line
	7750 1350 7700 1350
Wire Wire Line
	7700 1300 7700 1350
Connection ~ 7700 1350
Wire Wire Line
	7700 1350 7500 1350
$Comp
L Transistor_FET:2N7002 Q5
U 1 1 5CDAEC48
P 7300 2250
F 0 "Q5" V 7551 2250 50  0000 C CNN
F 1 "2N7002" V 7642 2250 50  0000 C CNN
F 2 "evend-adapter:SOT-23" H 7500 2175 50  0001 L CIN
F 3 "https://www.fairchildsemi.com/datasheets/2N/2N7002.pdf" H 7300 2250 50  0001 L CNN
	1    7300 2250
	0    1    1    0   
$EndComp
$Comp
L Device:R R13
U 1 1 5CDAEC52
P 7500 1950
F 0 "R13" V 7580 1950 50  0000 C CNN
F 1 "4k7" V 7500 1950 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7430 1950 50  0001 C CNN
F 3 "" H 7500 1950 50  0001 C CNN
	1    7500 1950
	0    1    1    0   
$EndComp
$Comp
L Device:R R12
U 1 1 5CDAEC5C
P 7100 1950
F 0 "R12" V 7180 1950 50  0000 C CNN
F 1 "10k" V 7100 1950 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7030 1950 50  0001 C CNN
F 3 "" H 7100 1950 50  0001 C CNN
	1    7100 1950
	0    1    1    0   
$EndComp
$Comp
L Device:R R23
U 1 1 5CDAEC66
P 7700 2150
F 0 "R23" V 7780 2150 50  0000 C CNN
F 1 "10k" V 7700 2150 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7630 2150 50  0001 C CNN
F 3 "" H 7700 2150 50  0001 C CNN
	1    7700 2150
	1    0    0    -1  
$EndComp
$Comp
L power:GND #PWR0122
U 1 1 5CDAEC70
P 6950 1950
F 0 "#PWR0122" H 6950 1700 50  0001 C CNN
F 1 "GND" H 6800 1900 50  0000 C CNN
F 2 "" H 6950 1950 50  0001 C CNN
F 3 "" H 6950 1950 50  0001 C CNN
	1    6950 1950
	1    0    0    -1  
$EndComp
Wire Wire Line
	7250 1950 7300 1950
Wire Wire Line
	7300 2050 7300 1950
Connection ~ 7300 1950
Wire Wire Line
	7300 1950 7350 1950
Wire Wire Line
	7700 1900 7700 1950
Wire Wire Line
	7650 1950 7700 1950
Connection ~ 7700 1950
Wire Wire Line
	7700 1950 7700 2000
Text GLabel 7100 2350 0    60   Input ~ 0
SCK-3V
Text GLabel 7750 2350 2    60   Input ~ 0
SCK-5V
Wire Wire Line
	7750 2350 7700 2350
Wire Wire Line
	7700 2300 7700 2350
Connection ~ 7700 2350
Wire Wire Line
	7700 2350 7500 2350
$Comp
L Transistor_FET:2N7002 Q6
U 1 1 5CDBD459
P 7300 3100
F 0 "Q6" V 7551 3100 50  0000 C CNN
F 1 "2N7002" V 7642 3100 50  0000 C CNN
F 2 "evend-adapter:SOT-23" H 7500 3025 50  0001 L CIN
F 3 "https://www.fairchildsemi.com/datasheets/2N/2N7002.pdf" H 7300 3100 50  0001 L CNN
	1    7300 3100
	0    1    1    0   
$EndComp
$Comp
L Device:R R15
U 1 1 5CDBD45F
P 7500 2800
F 0 "R15" V 7580 2800 50  0000 C CNN
F 1 "4k7" V 7500 2800 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7430 2800 50  0001 C CNN
F 3 "" H 7500 2800 50  0001 C CNN
	1    7500 2800
	0    1    1    0   
$EndComp
$Comp
L Device:R R14
U 1 1 5CDBD465
P 7100 2800
F 0 "R14" V 7180 2800 50  0000 C CNN
F 1 "10k" V 7100 2800 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7030 2800 50  0001 C CNN
F 3 "" H 7100 2800 50  0001 C CNN
	1    7100 2800
	0    1    1    0   
$EndComp
$Comp
L Device:R R24
U 1 1 5CDBD46B
P 7700 3000
F 0 "R24" V 7780 3000 50  0000 C CNN
F 1 "10k" V 7700 3000 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7630 3000 50  0001 C CNN
F 3 "" H 7700 3000 50  0001 C CNN
	1    7700 3000
	1    0    0    -1  
$EndComp
$Comp
L power:GND #PWR0124
U 1 1 5CDBD471
P 6950 2800
F 0 "#PWR0124" H 6950 2550 50  0001 C CNN
F 1 "GND" H 6800 2750 50  0000 C CNN
F 2 "" H 6950 2800 50  0001 C CNN
F 3 "" H 6950 2800 50  0001 C CNN
	1    6950 2800
	1    0    0    -1  
$EndComp
Wire Wire Line
	7250 2800 7300 2800
Wire Wire Line
	7300 2900 7300 2800
Connection ~ 7300 2800
Wire Wire Line
	7300 2800 7350 2800
Wire Wire Line
	7700 2750 7700 2800
Wire Wire Line
	7650 2800 7700 2800
Connection ~ 7700 2800
Wire Wire Line
	7700 2800 7700 2850
Text GLabel 6750 3200 0    60   Input ~ 0
SS-3V
Text GLabel 7750 3200 2    60   Input ~ 0
SS-5V
Wire Wire Line
	7750 3200 7700 3200
Wire Wire Line
	7700 3150 7700 3200
Connection ~ 7700 3200
Wire Wire Line
	7700 3200 7500 3200
$Comp
L Device:R R18
U 1 1 5CDC94AD
P 7600 3850
F 0 "R18" V 7680 3850 50  0000 C CNN
F 1 "10k" V 7600 3850 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7530 3850 50  0001 C CNN
F 3 "" H 7600 3850 50  0001 C CNN
	1    7600 3850
	0    -1   1    0   
$EndComp
$Comp
L Device:R R19
U 1 1 5CDC9883
P 7400 4050
F 0 "R19" V 7480 4050 50  0000 C CNN
F 1 "15k" V 7400 4050 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7330 4050 50  0001 C CNN
F 3 "" H 7400 4050 50  0001 C CNN
	1    7400 4050
	-1   0    0    -1  
$EndComp
$Comp
L power:GND #PWR0128
U 1 1 5CDCC236
P 7750 4250
F 0 "#PWR0128" H 7750 4000 50  0001 C CNN
F 1 "GND" H 7750 4100 50  0000 C CNN
F 2 "" H 7750 4250 50  0001 C CNN
F 3 "" H 7750 4250 50  0001 C CNN
	1    7750 4250
	-1   0    0    -1  
$EndComp
Wire Wire Line
	7450 3850 7400 3850
Wire Wire Line
	7400 3900 7400 3850
Connection ~ 7400 3850
Wire Wire Line
	7400 3850 7350 3850
Text GLabel 7350 3850 0    60   Input ~ 0
MISO-3V
Text GLabel 7750 3850 2    60   Input ~ 0
MISO-5V
Text GLabel 5850 5250 2    60   Input ~ 0
SS-3V
$Comp
L Device:R R20
U 1 1 5CDD1E52
P 7550 4600
F 0 "R20" V 7630 4600 50  0000 C CNN
F 1 "10k" V 7550 4600 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7480 4600 50  0001 C CNN
F 3 "" H 7550 4600 50  0001 C CNN
	1    7550 4600
	0    -1   1    0   
$EndComp
$Comp
L Device:R R21
U 1 1 5CDD1E58
P 7350 4800
F 0 "R21" V 7430 4800 50  0000 C CNN
F 1 "15k" V 7350 4800 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 7280 4800 50  0001 C CNN
F 3 "" H 7350 4800 50  0001 C CNN
	1    7350 4800
	-1   0    0    -1  
$EndComp
$Comp
L power:GND #PWR0129
U 1 1 5CDD1E5E
P 7350 4950
F 0 "#PWR0129" H 7350 4700 50  0001 C CNN
F 1 "GND" H 7350 4800 50  0000 C CNN
F 2 "" H 7350 4950 50  0001 C CNN
F 3 "" H 7350 4950 50  0001 C CNN
	1    7350 4950
	-1   0    0    -1  
$EndComp
Wire Wire Line
	7400 4600 7350 4600
Wire Wire Line
	7350 4650 7350 4600
Connection ~ 7350 4600
Wire Wire Line
	7350 4600 7300 4600
Text GLabel 7300 4600 0    60   Input ~ 0
INT-3V
Text GLabel 7700 4600 2    60   Input ~ 0
INT-5V
Text GLabel 1150 3250 0    60   Input ~ 0
INT-5V
Wire Wire Line
	1550 1000 1200 1000
Wire Wire Line
	1200 1000 1200 1200
$Comp
L evend-adapter-rescue:Orange_pi_lite J1
U 1 1 5CED487B
P 3800 4150
F 0 "J1" H 4725 4437 60  0000 C CNN
F 1 "Orange_pi_lite" H 4725 4331 60  0000 C CNN
F 2 "evend-adapter:Orange_pi_lite" H 4800 4200 60  0000 C CNN
F 3 "" H 3800 4150 60  0000 C CNN
	1    3800 4150
	1    0    0    -1  
$EndComp
$Comp
L power:GND #PWR0110
U 1 1 5CED45F5
P 2350 6000
F 0 "#PWR0110" H 2350 5750 50  0001 C CNN
F 1 "GND" V 2350 5800 50  0000 C CNN
F 2 "" H 2350 6000 50  0001 C CNN
F 3 "" H 2350 6000 50  0001 C CNN
	1    2350 6000
	0    -1   -1   0   
$EndComp
Text GLabel 3600 5250 0    60   Input ~ 0
SCK-3V
NoConn ~ 5850 4250
NoConn ~ 5850 5550
$Comp
L Device:R R5
U 1 1 5B090179
P 9650 2550
F 0 "R5" V 9730 2550 50  0000 C CNN
F 1 "680R" V 9650 2550 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 9580 2550 50  0001 C CNN
F 3 "" H 9650 2550 50  0001 C CNN
	1    9650 2550
	1    0    0    -1  
$EndComp
$Comp
L evend-adapter-rescue:Arduino_Pro_Mini-1 X1
U 1 1 5CF6010D
P 2000 3400
F 0 "X1" H 2000 4487 60  0000 C CNN
F 1 "Arduino_Pro_Mini-1" H 2000 4381 60  0000 C CNN
F 2 "evend-adapter:Arduino_Pro_Mini-1" H 2000 4381 60  0001 C CNN
F 3 "https://www.arduino.cc/en/Main/ArduinoBoardProMini" H 2000 4381 60  0001 C CNN
	1    2000 3400
	1    0    0    -1  
$EndComp
$Comp
L Device:R R2
U 1 1 5B08F6E8
P 9800 1450
F 0 "R2" V 9880 1450 50  0000 C CNN
F 1 "2K2" V 9800 1450 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 9730 1450 50  0001 C CNN
F 3 "" H 9800 1450 50  0001 C CNN
	1    9800 1450
	0    -1   -1   0   
$EndComp
$Comp
L power:GND #PWR0127
U 1 1 5CF649EE
P 3600 4550
F 0 "#PWR0127" H 3600 4300 50  0001 C CNN
F 1 "GND" V 3600 4350 50  0000 C CNN
F 2 "" H 3600 4550 50  0001 C CNN
F 3 "" H 3600 4550 50  0001 C CNN
	1    3600 4550
	0    1    1    0   
$EndComp
$Comp
L power:GND #PWR0130
U 1 1 5D0364F2
P 5850 4350
F 0 "#PWR0130" H 5850 4100 50  0001 C CNN
F 1 "GND" V 5850 4150 50  0000 C CNN
F 2 "" H 5850 4350 50  0001 C CNN
F 3 "" H 5850 4350 50  0001 C CNN
	1    5850 4350
	0    -1   -1   0   
$EndComp
$Comp
L evend-adapter-rescue:Conn_01x01_Female J5
U 1 1 5D048756
P 5400 2150
F 0 "J5" H 5428 2176 50  0000 L CNN
F 1 "Conn_01x01_Female" H 5428 2085 50  0000 L CNN
F 2 "evend-adapter:1pin" H 5400 2150 50  0001 C CNN
F 3 "~" H 5400 2150 50  0001 C CNN
	1    5400 2150
	1    0    0    -1  
$EndComp
$Comp
L evend-adapter-rescue:Conn_01x01_Female J4
U 1 1 5D049A0E
P 5400 1950
F 0 "J4" H 5428 1976 50  0000 L CNN
F 1 "Conn_01x01_Female" H 5428 1885 50  0000 L CNN
F 2 "evend-adapter:1pin" H 5400 1950 50  0001 C CNN
F 3 "~" H 5400 1950 50  0001 C CNN
	1    5400 1950
	1    0    0    -1  
$EndComp
$Comp
L power:GND #PWR0123
U 1 1 5D04B426
P 5200 2150
F 0 "#PWR0123" H 5200 1900 50  0001 C CNN
F 1 "GND" V 5200 1950 50  0000 C CNN
F 2 "" H 5200 2150 50  0001 C CNN
F 3 "" H 5200 2150 50  0001 C CNN
	1    5200 2150
	0    1    1    0   
$EndComp
$Comp
L Device:R o1
U 1 1 5CFF52FD
P 10350 1800
F 0 "o1" V 10430 1800 50  0000 C CNN
F 1 "0" V 10350 1800 50  0000 C CNN
F 2 "evend-adapter:C_1206_HandSoldering" V 10280 1800 50  0001 C CNN
F 3 "" H 10350 1800 50  0001 C CNN
	1    10350 1800
	-1   0    0    1   
$EndComp
$Comp
L power:GND #PWR0103
U 1 1 5B0943E7
P 10350 2500
F 0 "#PWR0103" H 10350 2250 50  0001 C CNN
F 1 "GND" H 10350 2350 50  0000 C CNN
F 2 "" H 10350 2500 50  0001 C CNN
F 3 "" H 10350 2500 50  0001 C CNN
	1    10350 2500
	1    0    0    -1  
$EndComp
Wire Wire Line
	10350 1950 10350 2050
$Comp
L Device:R o2
U 1 1 5D012E19
P 8050 900
F 0 "o2" V 8130 900 50  0000 C CNN
F 1 "0" V 8050 900 50  0000 C CNN
F 2 "evend-adapter:C_1206_HandSoldering" V 7980 900 50  0001 C CNN
F 3 "" H 8050 900 50  0001 C CNN
	1    8050 900 
	-1   0    0    1   
$EndComp
$Comp
L Device:R o3
U 1 1 5D02BB02
P 3200 3950
F 0 "o3" V 3280 3950 50  0000 C CNN
F 1 "0" V 3200 3950 50  0000 C CNN
F 2 "evend-adapter:C_1206_HandSoldering" V 3130 3950 50  0001 C CNN
F 3 "" H 3200 3950 50  0001 C CNN
	1    3200 3950
	0    -1   -1   0   
$EndComp
Wire Wire Line
	3050 3950 2550 3950
$Comp
L Device:R o4
U 1 1 5D05C350
P 7600 4250
F 0 "o4" V 7680 4250 50  0000 C CNN
F 1 "0" V 7600 4250 50  0000 C CNN
F 2 "evend-adapter:C_1206_HandSoldering" V 7530 4250 50  0001 C CNN
F 3 "" H 7600 4250 50  0001 C CNN
	1    7600 4250
	0    -1   -1   0   
$EndComp
Connection ~ 7300 950 
Wire Wire Line
	7600 950  7700 950 
Connection ~ 7700 950 
Wire Wire Line
	7700 950  7700 1000
Wire Wire Line
	7850 1050 7850 950 
Wire Wire Line
	7850 950  7700 950 
Wire Wire Line
	8050 1050 7850 1050
$Comp
L power:+5C #PWR0120
U 1 1 5D07A396
P 7700 850
F 0 "#PWR0120" H 7700 700 50  0001 C CNN
F 1 "+5C" H 7715 978 50  0000 L CNN
F 2 "" H 7700 850 50  0001 C CNN
F 3 "" H 7700 850 50  0001 C CNN
	1    7700 850 
	1    0    0    -1  
$EndComp
$Comp
L power:GND #PWR0133
U 1 1 5D0900EA
P 6950 950
F 0 "#PWR0133" H 6950 700 50  0001 C CNN
F 1 "GND" H 6800 900 50  0000 C CNN
F 2 "" H 6950 950 50  0001 C CNN
F 3 "" H 6950 950 50  0001 C CNN
	1    6950 950 
	1    0    0    -1  
$EndComp
$Comp
L power:GND #PWR0134
U 1 1 5D0906B4
P 7400 4300
F 0 "#PWR0134" H 7400 4050 50  0001 C CNN
F 1 "GND" H 7400 4150 50  0000 C CNN
F 2 "" H 7400 4300 50  0001 C CNN
F 3 "" H 7400 4300 50  0001 C CNN
	1    7400 4300
	-1   0    0    -1  
$EndComp
Wire Wire Line
	7400 4200 7400 4250
Wire Wire Line
	7450 4250 7400 4250
Connection ~ 7400 4250
Wire Wire Line
	7400 4250 7400 4300
$Comp
L Device:R o5
U 1 1 5D0DD97C
P 1300 3500
F 0 "o5" V 1380 3500 50  0000 C CNN
F 1 "0" V 1300 3500 50  0000 C CNN
F 2 "evend-adapter:C_1206_HandSoldering" V 1230 3500 50  0001 C CNN
F 3 "" H 1300 3500 50  0001 C CNN
	1    1300 3500
	0    -1   -1   0   
$EndComp
Wire Wire Line
	1450 3250 1150 3250
Wire Wire Line
	1450 3500 1450 3350
Connection ~ 1450 3250
Wire Wire Line
	1150 3500 1150 3350
Wire Wire Line
	1150 3350 1450 3350
Connection ~ 1450 3350
Wire Wire Line
	1450 3350 1450 3250
NoConn ~ 2350 6400
NoConn ~ 2350 6500
$Comp
L Device:R o6
U 1 1 5D116337
P 6950 3350
F 0 "o6" V 7030 3350 50  0000 C CNN
F 1 "0" V 6950 3350 50  0000 C CNN
F 2 "evend-adapter:C_1206_HandSoldering" V 6880 3350 50  0001 C CNN
F 3 "" H 6950 3350 50  0001 C CNN
	1    6950 3350
	0    -1   -1   0   
$EndComp
Wire Wire Line
	7100 3200 6800 3200
Connection ~ 7100 3200
Wire Wire Line
	6800 3350 6800 3200
Wire Wire Line
	6750 3200 6800 3200
Connection ~ 6800 3200
Wire Wire Line
	3850 2700 3850 2750
$Comp
L Device:R o7
U 1 1 5CFB5FA1
P 6950 3500
F 0 "o7" V 7030 3500 50  0000 C CNN
F 1 "0" V 6950 3500 50  0000 C CNN
F 2 "evend-adapter:C_1206_HandSoldering" V 6880 3500 50  0001 C CNN
F 3 "" H 6950 3500 50  0001 C CNN
	1    6950 3500
	0    -1   -1   0   
$EndComp
Wire Wire Line
	7100 3200 7100 3350
Connection ~ 7100 3350
Wire Wire Line
	7100 3350 7100 3500
Wire Wire Line
	6800 3500 6800 3350
Connection ~ 6800 3350
$Comp
L Device:R R6
U 1 1 5D17819D
P 9450 2750
F 0 "R6" V 9530 2750 50  0000 C CNN
F 1 "10k" V 9450 2750 50  0000 C CNN
F 2 "evend-adapter:C_0603_HandSoldering" V 9380 2750 50  0001 C CNN
F 3 "" H 9450 2750 50  0001 C CNN
	1    9450 2750
	0    1    1    0   
$EndComp
$Comp
L power:+5V #PWR0102
U 1 1 5DA08F9F
P 5200 1050
F 0 "#PWR0102" H 5200 900 50  0001 C CNN
F 1 "+5V" V 5200 1250 50  0000 C CNN
F 2 "" H 5200 1050 50  0001 C CNN
F 3 "" H 5200 1050 50  0001 C CNN
	1    5200 1050
	0    1    1    0   
$EndComp
$Comp
L power:+5V #PWR0106
U 1 1 5DA0A4F8
P 3850 2700
F 0 "#PWR0106" H 3850 2550 50  0001 C CNN
F 1 "+5V" H 3850 2900 50  0000 C CNN
F 2 "" H 3850 2700 50  0001 C CNN
F 3 "" H 3850 2700 50  0001 C CNN
	1    3850 2700
	1    0    0    -1  
$EndComp
$Comp
L power:+5V #PWR0109
U 1 1 5DA12A11
P 7700 1900
F 0 "#PWR0109" H 7700 1750 50  0001 C CNN
F 1 "+5V" H 7700 2100 50  0000 C CNN
F 2 "" H 7700 1900 50  0001 C CNN
F 3 "" H 7700 1900 50  0001 C CNN
	1    7700 1900
	1    0    0    -1  
$EndComp
$Comp
L power:+5V #PWR0111
U 1 1 5DA12C50
P 8050 750
F 0 "#PWR0111" H 8050 600 50  0001 C CNN
F 1 "+5V" H 8050 950 50  0000 C CNN
F 2 "" H 8050 750 50  0001 C CNN
F 3 "" H 8050 750 50  0001 C CNN
	1    8050 750 
	1    0    0    -1  
$EndComp
$Comp
L power:+5V #PWR0112
U 1 1 5DA1314C
P 10000 800
F 0 "#PWR0112" H 10000 650 50  0001 C CNN
F 1 "+5V" H 10000 1000 50  0000 C CNN
F 2 "" H 10000 800 50  0001 C CNN
F 3 "" H 10000 800 50  0001 C CNN
	1    10000 800 
	1    0    0    -1  
$EndComp
$Comp
L power:+5V #PWR0113
U 1 1 5DA13677
P 9650 2400
F 0 "#PWR0113" H 9650 2250 50  0001 C CNN
F 1 "+5V" H 9650 2600 50  0000 C CNN
F 2 "" H 9650 2400 50  0001 C CNN
F 3 "" H 9650 2400 50  0001 C CNN
	1    9650 2400
	1    0    0    -1  
$EndComp
$Comp
L power:+5V #PWR0115
U 1 1 5DA13B3F
P 7700 2750
F 0 "#PWR0115" H 7700 2600 50  0001 C CNN
F 1 "+5V" H 7700 2950 50  0000 C CNN
F 2 "" H 7700 2750 50  0001 C CNN
F 3 "" H 7700 2750 50  0001 C CNN
	1    7700 2750
	1    0    0    -1  
$EndComp
Wire Wire Line
	3500 2750 3700 2750
Wire Wire Line
	3700 2750 3850 2750
Connection ~ 3700 2750
Connection ~ 3850 2750
Wire Wire Line
	3850 2750 3850 3000
$Comp
L power:+5V #PWR0117
U 1 1 5DA1A493
P 2750 3050
F 0 "#PWR0117" H 2750 2900 50  0001 C CNN
F 1 "+5V" V 2750 3250 50  0000 C CNN
F 2 "" H 2750 3050 50  0001 C CNN
F 3 "" H 2750 3050 50  0001 C CNN
	1    2750 3050
	0    1    1    0   
$EndComp
$Comp
L power:+5V #PWR0119
U 1 1 5DA1A9F5
P 1950 5500
F 0 "#PWR0119" H 1950 5350 50  0001 C CNN
F 1 "+5V" H 1950 5700 50  0000 C CNN
F 2 "" H 1950 5500 50  0001 C CNN
F 3 "" H 1950 5500 50  0001 C CNN
	1    1950 5500
	1    0    0    -1  
$EndComp
NoConn ~ 2550 2750
Wire Wire Line
	1250 900  1550 900 
$Comp
L power:+5V #PWR0121
U 1 1 5DA2030B
P 5200 1950
F 0 "#PWR0121" H 5200 1800 50  0001 C CNN
F 1 "+5V" V 5200 2150 50  0000 C CNN
F 2 "" H 5200 1950 50  0001 C CNN
F 3 "" H 5200 1950 50  0001 C CNN
	1    5200 1950
	0    -1   -1   0   
$EndComp
NoConn ~ 5850 4150
Wire Wire Line
	7700 850  7700 950 
Wire Wire Line
	2350 3050 2550 3050
Connection ~ 2550 3050
Wire Wire Line
	2550 3050 2750 3050
Text GLabel 5500 7000 2    60   Input ~ 0
MOSI-3V
$Comp
L Driver_Display:CR2013-MI2120 U?
U 1 1 5E2A8BE3
P 4700 7000
F 0 "U?" H 4900 7600 50  0000 C CNN
F 1 "CR2013-MI2120" H 4350 7600 50  0000 C CNN
F 2 "Display:CR2013-MI2120" H 4700 6300 50  0001 C CNN
F 3 "http://pan.baidu.com/s/11Y990" H 4050 7500 50  0001 C CNN
	1    4700 7000
	1    0    0    -1  
$EndComp
Text GLabel 5500 7300 2    60   Input ~ 0
TFT-CS
Text GLabel 5500 7200 2    60   Input ~ 0
TFT-RESET
Text GLabel 5500 7100 2    60   Input ~ 0
TFT-DС
Text GLabel 5500 6800 2    60   Input ~ 0
TFT-LED
Text GLabel 5500 6900 2    60   Input ~ 0
SCK-3V
Text GLabel 3600 5950 0    60   Input ~ 0
TFT-CS
Text GLabel 3600 5850 0    60   Input ~ 0
TFT-RESET
Text GLabel 3600 5750 0    60   Input ~ 0
TFT-DС
Text GLabel 3600 5650 0    60   Input ~ 0
TFT-LED
NoConn ~ 3900 6600
NoConn ~ 3900 6700
NoConn ~ 3900 6800
NoConn ~ 3900 6900
NoConn ~ 3900 7000
NoConn ~ 5500 6700
$Comp
L power:+5V #PWR?
U 1 1 5E2BFA20
P 4700 6400
F 0 "#PWR?" H 4700 6250 50  0001 C CNN
F 1 "+5V" H 4700 6600 50  0000 C CNN
F 2 "" H 4700 6400 50  0001 C CNN
F 3 "" H 4700 6400 50  0001 C CNN
	1    4700 6400
	1    0    0    -1  
$EndComp
$Comp
L power:GND #PWR?
U 1 1 5E2C0269
P 4700 7600
F 0 "#PWR?" H 4700 7350 50  0001 C CNN
F 1 "GND" H 4700 7450 50  0000 C CNN
F 2 "" H 4700 7600 50  0001 C CNN
F 3 "" H 4700 7600 50  0001 C CNN
	1    4700 7600
	1    0    0    -1  
$EndComp
$EndSCHEMATC
