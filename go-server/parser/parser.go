package parser

import (
	"encoding/binary"
	"errors"
	"math"
)

// float32FromBytes mengonversi 4 byte Little Endian menjadi float32
func float32FromBytes(b []byte) float32 {
	bits := binary.LittleEndian.Uint32(b)
	return math.Float32frombits(bits)
}

// int32FromBytes mengonversi 4 byte Little Endian menjadi int32
func int32FromBytes(b []byte) int32 {
	return int32(binary.LittleEndian.Uint32(b))
}

// Parse mendekode paket byte mentah (min 323 bytes) menjadi TelemetryPacket.
func Parse(buf []byte) (*TelemetryPacket, error) {
	if len(buf) < 323 {
		return nil, errors.New("paket terlalu pendek, pastikan format Forza Data Out disetel ke 'Dash'")
	}

	p := &TelemetryPacket{}

	p.IsRaceOn = binary.LittleEndian.Uint32(buf[0:4]) != 0
	p.TimestampMs = binary.LittleEndian.Uint32(buf[4:8])
	p.EngineMaxRpm = float32FromBytes(buf[8:12])
	p.EngineIdleRpm = float32FromBytes(buf[12:16])
	p.CurrentEngineRpm = float32FromBytes(buf[16:20])
	
	p.AccelX = float32FromBytes(buf[20:24])
	p.AccelY = float32FromBytes(buf[24:28])
	p.AccelZ = float32FromBytes(buf[28:32])
	p.VelX = float32FromBytes(buf[32:36])
	p.VelY = float32FromBytes(buf[36:40])
	p.VelZ = float32FromBytes(buf[40:44])

	// Skip 12 bytes AngularVelocity
	p.Yaw = float32FromBytes(buf[56:60])
	p.Pitch = float32FromBytes(buf[60:64])
	p.Roll = float32FromBytes(buf[64:68])

	p.SuspensionFl = float32FromBytes(buf[68:72])
	p.SuspensionFr = float32FromBytes(buf[72:76])
	p.SuspensionRl = float32FromBytes(buf[76:80])
	p.SuspensionRr = float32FromBytes(buf[80:84])

	p.TireSlipRatioFl = float32FromBytes(buf[84:88])
	p.TireSlipRatioFr = float32FromBytes(buf[88:92])
	p.TireSlipRatioRl = float32FromBytes(buf[92:96])
	p.TireSlipRatioRr = float32FromBytes(buf[96:100])

	// Skip Wheel Rotation, Rumble, Puddle, Surface (Total 64 bytes)
	p.TireSlipAngleFl = float32FromBytes(buf[164:168])
	p.TireSlipAngleFr = float32FromBytes(buf[168:172])
	p.TireSlipAngleRl = float32FromBytes(buf[172:176])
	p.TireSlipAngleRr = float32FromBytes(buf[176:180])

	// Skip CombinedSlip & SuspensionTravelMeters (Total 32 bytes)
	p.CarOrdinal = int32FromBytes(buf[212:216])
	p.CarClass = int32FromBytes(buf[216:220])
	p.CarPi = int32FromBytes(buf[220:224])
	p.DrivetrainType = int32FromBytes(buf[224:228])
	p.NumCylinders = int32FromBytes(buf[228:232])

	// Skip 12 bytes Unknown
	p.PositionX = float32FromBytes(buf[244:248])
	p.PositionY = float32FromBytes(buf[248:252])
	p.PositionZ = float32FromBytes(buf[252:256])
	p.SpeedMs = float32FromBytes(buf[256:260])
	p.Power = float32FromBytes(buf[260:264])
	p.Torque = float32FromBytes(buf[264:268])

	// Konversi suhu ban dari Fahrenheit ke Celsius langsung di Parser
	p.TireTempFl = (float32FromBytes(buf[268:272]) - 32.0) * 5.0 / 9.0
	p.TireTempFr = (float32FromBytes(buf[272:276]) - 32.0) * 5.0 / 9.0
	p.TireTempRl = (float32FromBytes(buf[276:280]) - 32.0) * 5.0 / 9.0
	p.TireTempRr = (float32FromBytes(buf[280:284]) - 32.0) * 5.0 / 9.0

	p.Boost = float32FromBytes(buf[284:288])
	p.Fuel = float32FromBytes(buf[288:292])
	p.DistanceTraveled = float32FromBytes(buf[292:296])
	p.BestLap = float32FromBytes(buf[296:300])
	p.LastLap = float32FromBytes(buf[300:304])
	p.CurrentLap = float32FromBytes(buf[304:308])
	p.CurrentRaceTime = float32FromBytes(buf[308:312])

	p.LapNumber = binary.LittleEndian.Uint16(buf[312:314])
	p.RacePosition = buf[314]
	p.Throttle = buf[315]
	p.Brake = buf[316]
	p.Clutch = buf[317]
	p.Handbrake = buf[318]
	p.Gear = buf[319]
	p.Steer = int8(buf[320])
	p.DrivingLine = int8(buf[321])
	p.AiBrakeDiff = int8(buf[322])

	return p, nil
}
