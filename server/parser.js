export class ParseError extends Error {
  constructor(message) {
    super(message);
    this.name = 'ParseError';
  }
}

export function parse(buf) {
  if (buf.length < 323) {
    throw new ParseError(`packet too short: ${buf.length} bytes (need ≥323)`);
  }

  // Sled fields (bytes 0–231)
  const isRaceOn = buf.readInt32LE(0) !== 0;
  const timestampMs = buf.readUInt32LE(4);
  const engineMaxRpm = buf.readFloatLE(8);
  const engineIdleRpm = buf.readFloatLE(12);
  const currentEngineRpm = buf.readFloatLE(16);
  const accelX = buf.readFloatLE(20);
  const accelY = buf.readFloatLE(24);
  const accelZ = buf.readFloatLE(28);
  const velX = buf.readFloatLE(32);
  const velY = buf.readFloatLE(36);
  const velZ = buf.readFloatLE(40);
  
  // skip 3 floats AngularVelocity X/Y/Z (44, 48, 52)
  const yaw = buf.readFloatLE(56);
  const pitch = buf.readFloatLE(60);
  const roll = buf.readFloatLE(64);
  const suspensionFl = buf.readFloatLE(68);
  const suspensionFr = buf.readFloatLE(72);
  const suspensionRl = buf.readFloatLE(76);
  const suspensionRr = buf.readFloatLE(80);
  const tireSlipRatioFl = buf.readFloatLE(84);
  const tireSlipRatioFr = buf.readFloatLE(88);
  const tireSlipRatioRl = buf.readFloatLE(92);
  const tireSlipRatioRr = buf.readFloatLE(96);

  // skip 4 floats WheelRotationSpeed (100, 104, 108, 112)
  // skip 4 floats WheelOnRumbleStrip (116, 120, 124, 128)
  // skip 4 floats WheelInPuddleDepth (132, 136, 140, 144)
  // skip 4 floats SurfaceRumble (148, 152, 156, 160)

  const tireSlipAngleFl = buf.readFloatLE(164);
  const tireSlipAngleFr = buf.readFloatLE(168);
  const tireSlipAngleRl = buf.readFloatLE(172);
  const tireSlipAngleRr = buf.readFloatLE(176);

  // skip 4 floats TireCombinedSlip (180, 184, 188, 192)
  // skip 4 floats SuspensionTravelMeters (196, 200, 204, 208)

  const carOrdinal = buf.readInt32LE(212);
  const carClass = buf.readInt32LE(216);
  const carPi = buf.readInt32LE(220);
  const drivetrainType = buf.readInt32LE(224);
  const numCylinders = buf.readInt32LE(228);

  // Dash-only fields
  // skip bytes 232–243 (12 bytes)
  const positionX = buf.readFloatLE(244);
  const positionY = buf.readFloatLE(248);
  const positionZ = buf.readFloatLE(252);
  const speedMs = buf.readFloatLE(256);
  const power = buf.readFloatLE(260);
  const torque = buf.readFloatLE(264);

  // Game sends tire temps in Fahrenheit; convert to Celsius for display
  const tireTempFl = (buf.readFloatLE(268) - 32.0) * 5.0 / 9.0;
  const tireTempFr = (buf.readFloatLE(272) - 32.0) * 5.0 / 9.0;
  const tireTempRl = (buf.readFloatLE(276) - 32.0) * 5.0 / 9.0;
  const tireTempRr = (buf.readFloatLE(280) - 32.0) * 5.0 / 9.0;

  const boost = buf.readFloatLE(284);
  const fuel = buf.readFloatLE(288);
  const distanceTraveled = buf.readFloatLE(292);
  const bestLap = buf.readFloatLE(296);
  const lastLap = buf.readFloatLE(300);
  const currentLap = buf.readFloatLE(304);
  const currentRaceTime = buf.readFloatLE(308);
  
  const lapNumber = buf.readUInt16LE(312);
  const racePosition = buf.readUInt8(314);
  const throttle = buf.readUInt8(315);
  const brake = buf.readUInt8(316);
  const clutch = buf.readUInt8(317);
  const handbrake = buf.readUInt8(318);
  const gear = buf.readUInt8(319);
  const steer = buf.readInt8(320);
  const drivingLine = buf.readInt8(321);
  const aiBrakeDiff = buf.readInt8(322);

  // Optional tire wear (bytes 323+)
  let tireWearFl = null, tireWearFr = null, tireWearRl = null, tireWearRr = null;
  if (buf.length >= 327) tireWearFl = buf.readFloatLE(323);
  if (buf.length >= 331) tireWearFr = buf.readFloatLE(327);
  if (buf.length >= 335) tireWearRl = buf.readFloatLE(331);
  if (buf.length >= 339) tireWearRr = buf.readFloatLE(335);

  return {
    isRaceOn,
    timestampMs,
    engineMaxRpm,
    engineIdleRpm,
    currentEngineRpm,
    accelX,
    accelY,
    accelZ,
    velX,
    velY,
    velZ,
    positionX,
    positionY,
    positionZ,
    tireSlipRatioFl,
    tireSlipRatioFr,
    tireSlipRatioRl,
    tireSlipRatioRr,
    tireSlipAngleFl,
    tireSlipAngleFr,
    tireSlipAngleRl,
    tireSlipAngleRr,
    carOrdinal,
    carClass,
    carPi,
    drivetrainType,
    speedMs,
    power,
    torque,
    tireTempFl,
    tireTempFr,
    tireTempRl,
    tireTempRr,
    boost,
    fuel,
    distanceTraveled,
    bestLap,
    lastLap,
    currentLap,
    currentRaceTime,
    lapNumber,
    racePosition,
    throttle,
    brake,
    clutch,
    handbrake,
    gear,
    steer,
    yaw,
    pitch,
    roll,
    suspensionFl,
    suspensionFr,
    suspensionRl,
    suspensionRr,
    tireWearFl,
    tireWearFr,
    tireWearRl,
    tireWearRr,
  };
}
