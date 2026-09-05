import * as THREE from 'three';
import { mulberry32 } from './noise.js';
import { WATER_Y } from './terrain.js';

// Everything that sits on (or above) the terrain: scattered vegetation,
// water, the village, points of interest, floating islands, clouds, and the
// keyframe-animated bits (airship, windmill) in the spirit of the three.js
// "animation / keyframes" example.

// Flat pads the terrain is blended toward so landmarks sit level. Each named
// pad is also a discoverable point of interest for the HUD.
export const PADS = [
  { name: 'Harbourlight Village', x: 20, z: 30, r: 34, minY: 4 },
  { name: 'The Old Lighthouse', x: -168, z: 96, r: 13, minY: 9 },
  { name: 'Pinwheel Mill', x: 112, z: -124, r: 17, minY: 8 },
  { name: 'The Standing Stones', x: -62, z: -152, r: 20, minY: 12 },
  { name: 'Torii Shrine', x: 152, z: 86, r: 15, minY: 5 },
  { name: 'Driftwood Dock', x: 72, z: 176, r: 12, y: 1.6 },
];

const PALETTE = {
  trunk: 0x8b5e3c,
  conifer: [0x4f9d69, 0x6bb77b, 0x3f8f7a, 0x7cc48c],
  broadleaf: [0xf29ca3, 0xf6b26b, 0x8fd694, 0x9dd9d2, 0xffd166, 0xc7e08c],
  rock: [0xa8a3b0, 0x9a95a8, 0xb9b3c2],
  flowers: [0xff8fab, 0xffd166, 0xffffff, 0xa0c4ff, 0xffa06b],
  walls: [0xf7e1c5, 0xf3b6c4, 0xbfe2f0, 0xfff3b0, 0xd7f0d2, 0xe6d3f5],
  roofs: [0xd96c6c, 0x6c8fd9, 0x5aa88a, 0xe0925b, 0x8b6cd9],
  wood: 0xa97c50,
  stone: 0xb8b3bf,
};

const mat = (color, extra = {}) =>
  new THREE.MeshStandardMaterial({ color, flatShading: true, roughness: 0.9, metalness: 0, ...extra });
const glow = (color, intensity = 1.6) =>
  new THREE.MeshStandardMaterial({ color, emissive: color, emissiveIntensity: intensity, roughness: 0.6 });

function shadowed(mesh, cast = true, receive = true) {
  mesh.castShadow = cast;
  mesh.receiveShadow = receive;
  return mesh;
}

function pick(rng, arr) {
  return arr[Math.floor(rng() * arr.length)];
}

export function buildWorld(scene, terrain, seed = 7) {
  const rng = mulberry32(seed * 977 + 13);
  const colliders = { boxes: [], circles: [], platforms: [] };
  const updaters = [];
  const mixers = [];
  const pois = PADS.map((p) => ({ name: p.name, x: p.x, z: p.z, r: p.r + 8 }));

  const isOnPad = (x, z, slack = 1.15) =>
    PADS.some((p) => Math.hypot(x - p.x, z - p.z) < p.r * slack);

  buildWater(scene, terrain, updaters);
  scatterTrees(scene, terrain, rng, colliders, isOnPad);
  scatterRocks(scene, terrain, rng, colliders, isOnPad);
  scatterFlowers(scene, terrain, rng, isOnPad);

  buildVillage(scene, terrain, rng, colliders, PADS[0]);
  buildLighthouse(scene, terrain, colliders, PADS[1]);
  buildWindmill(scene, terrain, colliders, PADS[2], mixers);
  buildStandingStones(scene, terrain, rng, colliders, PADS[3]);
  buildShrine(scene, terrain, colliders, PADS[4]);
  buildDock(scene, terrain, colliders, PADS[5], updaters);

  buildFloatingIslands(scene, rng, updaters);
  buildClouds(scene, rng, updaters);
  buildAirship(scene, mixers);
  buildBirds(scene, updaters, PADS[0]);
  pois.push({ name: 'Snowcap Summit', x: -40, z: -70, r: 26 });

  return {
    colliders,
    pois,
    update(dt, t) {
      for (const u of updaters) u(dt, t);
      for (const m of mixers) m.update(dt);
    },
  };
}

// --- Water -------------------------------------------------------------------

function buildWater(scene, terrain, updaters) {
  const waterMat = new THREE.MeshStandardMaterial({
    color: 0x5cc3e8,
    transparent: true,
    opacity: 0.8,
    roughness: 0.55,
    metalness: 0,
    flatShading: true,
  });
  // Inner animated sheet with visible facets…
  const inner = new THREE.PlaneGeometry(terrain.size + 40, terrain.size + 40, 70, 70);
  inner.rotateX(-Math.PI / 2);
  const innerMesh = new THREE.Mesh(inner, waterMat);
  innerMesh.position.y = WATER_Y;
  innerMesh.receiveShadow = true;
  scene.add(innerMesh);
  // …and a huge flat sheet out to the fog for the horizon.
  const outer = new THREE.Mesh(new THREE.PlaneGeometry(6000, 6000), waterMat);
  outer.rotateX(-Math.PI / 2);
  outer.position.y = WATER_Y - 0.05;
  scene.add(outer);

  const pos = inner.attributes.position;
  const base = pos.array.slice();
  updaters.push((dt, t) => {
    for (let i = 0; i < pos.count; i++) {
      const x = base[i * 3], z = base[i * 3 + 2];
      pos.array[i * 3 + 1] = Math.sin(x * 0.18 + t * 0.9) * 0.18 + Math.cos(z * 0.15 + t * 0.7) * 0.18;
    }
    pos.needsUpdate = true;
  });
}

// --- Vegetation & rocks --------------------------------------------------------

function scatterTrees(scene, terrain, rng, colliders, isOnPad) {
  const conifers = [];
  const broadleafs = [];
  const half = terrain.size * 0.5 - 6;
  const n = new THREE.Vector3();
  const density = terrain.noise; // reuse the terrain noise for forest clumps
  let tries = 0;
  while (conifers.length + broadleafs.length < 1700 && tries < 40000) {
    tries++;
    const x = (rng() * 2 - 1) * half;
    const z = (rng() * 2 - 1) * half;
    const h = terrain.heightAt(x, z);
    if (h < WATER_Y + 1.6 || h > 21) continue;
    terrain.normalAt(x, z, n);
    if (n.y < 0.78) continue;
    if (isOnPad(x, z)) continue;
    const d = density.fbm(x * 0.012 + 100, z * 0.012 + 100, 2);
    if (d < -0.05 && rng() > 0.15) continue;
    const s = 0.8 + rng() * 0.7;
    const t = { x, y: h - 0.2, z, s, rot: rng() * Math.PI * 2 };
    (h > 12 || d > 0.35 ? conifers : broadleafs).push(t);
    colliders.circles.push({ x, z, r: 0.45 * s });
  }

  const trunkGeo = new THREE.CylinderGeometry(0.18, 0.3, 1, 5);
  trunkGeo.translate(0, 0.5, 0);
  const all = conifers.concat(broadleafs);
  const trunks = new THREE.InstancedMesh(trunkGeo, mat(PALETTE.trunk), all.length);
  trunks.castShadow = true;
  trunks.receiveShadow = true;
  const m = new THREE.Matrix4();
  const q = new THREE.Quaternion();
  const up = new THREE.Vector3(0, 1, 0);
  const sc = new THREE.Vector3();
  all.forEach((t, i) => {
    q.setFromAxisAngle(up, t.rot);
    const trunkH = 1.4 * t.s;
    m.compose(new THREE.Vector3(t.x, t.y, t.z), q, sc.set(t.s, trunkH, t.s));
    trunks.setMatrixAt(i, m);
  });
  scene.add(trunks);

  const color = new THREE.Color();

  // Conifers: two stacked cones.
  const coneLo = new THREE.ConeGeometry(1.35, 2.6, 6);
  coneLo.translate(0, 1.4 + 1.3, 0);
  const coneHi = new THREE.ConeGeometry(0.9, 2.2, 6);
  coneHi.translate(0, 1.4 + 2.9, 0);
  for (const geo of [coneLo, coneHi]) {
    const im = new THREE.InstancedMesh(geo, mat(0xffffff), conifers.length);
    im.castShadow = true;
    im.receiveShadow = true;
    conifers.forEach((t, i) => {
      q.setFromAxisAngle(up, t.rot);
      m.compose(new THREE.Vector3(t.x, t.y, t.z), q, sc.set(t.s, t.s, t.s));
      im.setMatrixAt(i, m);
      color.setHex(PALETTE.conifer[i % PALETTE.conifer.length]);
      im.setColorAt(i, color);
    });
    scene.add(im);
  }

  // Broadleaf: a faceted blob.
  const blob = new THREE.IcosahedronGeometry(1.6, 0);
  blob.scale(1, 1.15, 1);
  blob.translate(0, 1.4 + 1.5, 0);
  const im = new THREE.InstancedMesh(blob, mat(0xffffff), broadleafs.length);
  im.castShadow = true;
  im.receiveShadow = true;
  broadleafs.forEach((t, i) => {
    q.setFromAxisAngle(up, t.rot);
    m.compose(new THREE.Vector3(t.x, t.y, t.z), q, sc.set(t.s, t.s, t.s));
    im.setMatrixAt(i, m);
    color.setHex(pick(rng, PALETTE.broadleaf));
    im.setColorAt(i, color);
  });
  scene.add(im);
}

function scatterRocks(scene, terrain, rng, colliders, isOnPad) {
  const rocks = [];
  const half = terrain.size * 0.5 - 6;
  let tries = 0;
  while (rocks.length < 320 && tries < 8000) {
    tries++;
    const x = (rng() * 2 - 1) * half;
    const z = (rng() * 2 - 1) * half;
    const h = terrain.heightAt(x, z);
    if (h < WATER_Y - 0.5 || isOnPad(x, z, 1.0)) continue;
    const s = 0.6 + rng() * rng() * 2.4;
    rocks.push({ x, y: h - s * 0.35, z, s });
    if (s > 1.2) colliders.circles.push({ x, z, r: s * 0.8 });
  }
  const geo = new THREE.DodecahedronGeometry(1, 0);
  const im = new THREE.InstancedMesh(geo, mat(0xffffff), rocks.length);
  im.castShadow = true;
  im.receiveShadow = true;
  const m = new THREE.Matrix4();
  const e = new THREE.Euler();
  const q = new THREE.Quaternion();
  const color = new THREE.Color();
  rocks.forEach((r, i) => {
    e.set(rng() * Math.PI, rng() * Math.PI, rng() * Math.PI);
    q.setFromEuler(e);
    m.compose(new THREE.Vector3(r.x, r.y, r.z), q, new THREE.Vector3(r.s, r.s * (0.6 + rng() * 0.6), r.s));
    im.setMatrixAt(i, m);
    color.setHex(pick(rng, PALETTE.rock));
    im.setColorAt(i, color);
  });
  scene.add(im);
}

function scatterFlowers(scene, terrain, rng, isOnPad) {
  const pts = [];
  const half = terrain.size * 0.5 - 6;
  let tries = 0;
  while (pts.length < 2600 && tries < 20000) {
    tries++;
    const x = (rng() * 2 - 1) * half;
    const z = (rng() * 2 - 1) * half;
    const h = terrain.heightAt(x, z);
    if (h < WATER_Y + 1.2 || h > 18) continue;
    if (isOnPad(x, z, 0.9) && rng() > 0.35) continue;
    if (Math.hypot(x - PADS[0].x, z - PADS[0].z) < 10.5) continue;
    pts.push({ x, y: h + 0.15, z });
  }
  const geo = new THREE.OctahedronGeometry(0.22, 0);
  geo.scale(1, 0.7, 1);
  const im = new THREE.InstancedMesh(geo, mat(0xffffff, { roughness: 0.7 }), pts.length);
  const m = new THREE.Matrix4();
  const color = new THREE.Color();
  pts.forEach((p, i) => {
    m.makeTranslation(p.x, p.y, p.z);
    im.setMatrixAt(i, m);
    color.setHex(pick(rng, PALETTE.flowers));
    im.setColorAt(i, color);
  });
  scene.add(im);
}

// --- Builders shared by landmarks --------------------------------------------

function addBoxCollider(colliders, x, z, hw, hd, rot = 0) {
  colliders.boxes.push({ x, z, hw, hd, rot });
}

function makeHouse(rng, w, d, h, wallColor, roofColor) {
  const g = new THREE.Group();
  const walls = shadowed(new THREE.Mesh(new THREE.BoxGeometry(w, h, d), mat(wallColor)));
  walls.position.y = h / 2;
  g.add(walls);

  // Gable roof: extruded triangle.
  const over = 1.18;
  const roofH = 0.55 * w;
  const shape = new THREE.Shape();
  shape.moveTo(-w * over / 2, 0);
  shape.lineTo(w * over / 2, 0);
  shape.lineTo(0, roofH);
  shape.closePath();
  const roofGeo = new THREE.ExtrudeGeometry(shape, { depth: d * over, bevelEnabled: false });
  roofGeo.translate(0, 0, -d * over / 2);
  const roof = shadowed(new THREE.Mesh(roofGeo, mat(roofColor)));
  roof.position.y = h - 0.05;
  g.add(roof);

  const door = new THREE.Mesh(new THREE.BoxGeometry(0.9, 1.7, 0.12), mat(0x6b4a2f));
  door.position.set(w * 0.15, 0.85, d / 2 + 0.05);
  g.add(door);

  const winGeo = new THREE.BoxGeometry(0.7, 0.7, 0.1);
  const winMat = glow(0xfff1b0, 1.4);
  const nWin = 1 + Math.floor(rng() * 2);
  for (let i = 0; i < nWin; i++) {
    const win = new THREE.Mesh(winGeo, winMat);
    win.position.set(-w * 0.28 + i * 0.9 - (nWin - 1) * 0.45, h * 0.62, d / 2 + 0.05);
    g.add(win);
  }
  const sideWin = new THREE.Mesh(winGeo, winMat);
  sideWin.rotation.y = Math.PI / 2;
  sideWin.position.set(w / 2 + 0.05, h * 0.62, 0);
  g.add(sideWin);

  if (rng() > 0.4) {
    const chimney = shadowed(new THREE.Mesh(new THREE.BoxGeometry(0.6, 1.4, 0.6), mat(0x9a8a86)));
    chimney.position.set(-w * 0.3, h + roofH * 0.55, -d * 0.2);
    g.add(chimney);
  }
  return g;
}

function makeLamp(withLight = true) {
  const g = new THREE.Group();
  const pole = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(0.08, 0.12, 3.2, 6), mat(0x3f3a4a)));
  pole.position.y = 1.6;
  g.add(pole);
  const bulb = new THREE.Mesh(new THREE.SphereGeometry(0.3, 8, 6), glow(0xffd28a, 2.2));
  bulb.position.y = 3.4;
  g.add(bulb);
  if (withLight) {
    const l = new THREE.PointLight(0xffc98a, 14, 16, 2);
    l.position.y = 3.3;
    g.add(l);
  }
  return g;
}

function makeSmallTree(color, s = 1) {
  const g = new THREE.Group();
  const trunk = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(0.15, 0.25, 1.2, 5), mat(PALETTE.trunk)));
  trunk.position.y = 0.6;
  g.add(trunk);
  const top = shadowed(new THREE.Mesh(new THREE.IcosahedronGeometry(1.3, 0), mat(color)));
  top.position.y = 2.1;
  top.scale.set(1, 1.15, 1);
  g.add(top);
  g.scale.setScalar(s);
  return g;
}

// --- Landmarks -----------------------------------------------------------------

function buildVillage(scene, terrain, rng, colliders, pad) {
  const g = new THREE.Group();
  const cx = pad.x, cz = pad.z;

  // Plaza with a fountain.
  const plaza = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(9, 9, 0.3, 12), mat(0xd9cfc1)), false);
  plaza.position.set(cx, pad.y, cz);
  g.add(plaza);
  const basin = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(2.6, 2.8, 0.9, 10), mat(PALETTE.stone)));
  basin.position.set(cx, pad.y + 0.5, cz);
  g.add(basin);
  const pool = new THREE.Mesh(new THREE.CylinderGeometry(2.3, 2.3, 0.1, 10), mat(0x7fd3ee, { roughness: 0.2 }));
  pool.position.set(cx, pad.y + 0.9, cz);
  g.add(pool);
  const spire = shadowed(new THREE.Mesh(new THREE.ConeGeometry(0.5, 2.2, 6), mat(PALETTE.stone)));
  spire.position.set(cx, pad.y + 2, cz);
  g.add(spire);
  addBoxCollider(colliders, cx, cz, 2.9, 2.9);

  // Houses in a loose ring, facing the plaza.
  const count = 10;
  for (let i = 0; i < count; i++) {
    const a = (i / count) * Math.PI * 2 + rng() * 0.25;
    const r = 15 + rng() * 9;
    const x = cx + Math.cos(a) * r;
    const z = cz + Math.sin(a) * r;
    const w = 4 + rng() * 2.5;
    const d = 4 + rng() * 2.5;
    const h = 2.8 + rng() * 1.6;
    const house = makeHouse(rng, w, d, h, pick(rng, PALETTE.walls), pick(rng, PALETTE.roofs));
    const rot = Math.atan2(cx - x, cz - z); // +z face (door) toward the plaza
    house.rotation.y = rot;
    house.position.set(x, terrain.heightAt(x, z) - 0.15, z);
    g.add(house);
    addBoxCollider(colliders, x, z, w / 2 + 0.2, d / 2 + 0.2, rot);
  }

  // Lamps around the plaza (a few real lights, the rest just glow).
  for (let i = 0; i < 6; i++) {
    const a = (i / 6) * Math.PI * 2 + 0.3;
    const x = cx + Math.cos(a) * 10.5;
    const z = cz + Math.sin(a) * 10.5;
    const lamp = makeLamp(i % 2 === 0);
    lamp.position.set(x, terrain.heightAt(x, z), z);
    g.add(lamp);
    colliders.circles.push({ x, z, r: 0.3 });
  }

  // A few ornamental trees.
  for (let i = 0; i < 7; i++) {
    const a = rng() * Math.PI * 2;
    const r = 26 + rng() * 6;
    const x = cx + Math.cos(a) * r;
    const z = cz + Math.sin(a) * r;
    const t = makeSmallTree(pick(rng, PALETTE.broadleaf), 1.1 + rng() * 0.5);
    t.position.set(x, terrain.heightAt(x, z) - 0.1, z);
    g.add(t);
    colliders.circles.push({ x, z, r: 0.4 });
  }

  scene.add(g);
}

function buildLighthouse(scene, terrain, colliders, pad) {
  const g = new THREE.Group();
  const y0 = terrain.heightAt(pad.x, pad.z) - 0.2;
  const base = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(4.5, 5, 1, 10), mat(PALETTE.stone)));
  base.position.y = 0.5;
  g.add(base);
  const bands = [0xf7f4ee, 0xe0574f, 0xf7f4ee, 0xe0574f, 0xf7f4ee];
  let y = 1;
  bands.forEach((c, i) => {
    const rTop = 2.2 - i * 0.22, rBot = 2.42 - i * 0.22, h = 3.2;
    const seg = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(rTop, rBot, h, 10), mat(c)));
    seg.position.y = y + h / 2;
    g.add(seg);
    y += h;
  });
  const gallery = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(1.9, 1.5, 0.5, 10), mat(0x3f3a4a)));
  gallery.position.y = y + 0.25;
  g.add(gallery);
  const lamp = new THREE.Mesh(new THREE.CylinderGeometry(1.1, 1.1, 1.8, 10), glow(0xfff2b0, 2.5));
  lamp.position.y = y + 1.4;
  g.add(lamp);
  const cap = shadowed(new THREE.Mesh(new THREE.ConeGeometry(1.6, 1.6, 10), mat(0xe0574f)));
  cap.position.y = y + 3.1;
  g.add(cap);
  const light = new THREE.PointLight(0xfff0c0, 30, 60, 1.5);
  light.position.y = y + 1.4;
  g.add(light);

  // Keeper's cottage.
  const cottage = makeHouse(mulberry32(3), 4.2, 3.6, 2.8, 0xf7e1c5, 0xe0574f);
  cottage.position.set(8, terrain.heightAt(pad.x + 8, pad.z + 2) - y0 - 0.1, 2);
  cottage.rotation.y = -Math.PI / 2;
  g.add(cottage);
  addBoxCollider(colliders, pad.x + 8, pad.z + 2, 2.3, 2.0, -Math.PI / 2);

  g.position.set(pad.x, y0, pad.z);
  scene.add(g);
  colliders.circles.push({ x: pad.x, z: pad.z, r: 2.6 });
}

function buildWindmill(scene, terrain, colliders, pad, mixers) {
  const g = new THREE.Group();
  const y0 = terrain.heightAt(pad.x, pad.z) - 0.2;
  const tower = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(2.4, 3.4, 9, 8), mat(0xf3e6d2)));
  tower.position.y = 4.5;
  g.add(tower);
  const roof = shadowed(new THREE.Mesh(new THREE.ConeGeometry(3, 2.6, 8), mat(0x6c8fd9)));
  roof.position.y = 10.3;
  g.add(roof);
  const door = new THREE.Mesh(new THREE.BoxGeometry(1.1, 2, 0.15), mat(0x6b4a2f));
  door.position.set(0, 1, 3.35);
  g.add(door);
  const win = new THREE.Mesh(new THREE.BoxGeometry(0.8, 0.8, 0.12), glow(0xfff1b0, 1.4));
  win.position.set(0, 5.5, 2.75);
  g.add(win);

  // Hub + four sails, turned by a keyframe clip.
  const hub = new THREE.Group();
  hub.position.set(0, 8.2, 3.2);
  const axle = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(0.3, 0.3, 1.2, 6), mat(0x6b4a2f)));
  axle.rotation.x = Math.PI / 2;
  hub.add(axle);
  for (let i = 0; i < 4; i++) {
    const arm = new THREE.Group();
    arm.rotation.z = (i * Math.PI) / 2;
    const spar = shadowed(new THREE.Mesh(new THREE.BoxGeometry(0.25, 7, 0.2), mat(0x6b4a2f)));
    spar.position.y = 3.5;
    arm.add(spar);
    const sail = shadowed(new THREE.Mesh(new THREE.BoxGeometry(1.5, 5.5, 0.06), mat(0xfaf6ee)));
    sail.position.set(0.9, 4.2, 0.1);
    arm.add(sail);
    hub.add(arm);
  }
  g.add(hub);

  const times = [0, 3, 6, 9, 12];
  const q = new THREE.Quaternion();
  const axis = new THREE.Vector3(0, 0, 1);
  const values = [];
  times.forEach((_, i) => {
    q.setFromAxisAngle(axis, -(i * Math.PI) / 2);
    values.push(q.x, q.y, q.z, q.w);
  });
  const clip = new THREE.AnimationClip('sails', 12, [new THREE.QuaternionKeyframeTrack('.quaternion', times, values)]);
  const mixer = new THREE.AnimationMixer(hub);
  mixer.clipAction(clip).play();
  mixers.push(mixer);

  g.position.set(pad.x, y0, pad.z);
  scene.add(g);
  colliders.circles.push({ x: pad.x, z: pad.z, r: 3.6 });
}

function buildStandingStones(scene, terrain, rng, colliders, pad) {
  const g = new THREE.Group();
  const n = 11;
  for (let i = 0; i < n; i++) {
    const a = (i / n) * Math.PI * 2;
    const r = 11;
    const x = pad.x + Math.cos(a) * r;
    const z = pad.z + Math.sin(a) * r;
    const h = 3.5 + rng() * 2.5;
    const stone = shadowed(new THREE.Mesh(new THREE.BoxGeometry(1.4, h, 0.9), mat(0x9d97a8)));
    stone.position.set(x, terrain.heightAt(x, z) + h / 2 - 0.4, z);
    stone.rotation.set((rng() - 0.5) * 0.12, a + Math.PI / 2, (rng() - 0.5) * 0.12);
    g.add(stone);
    addBoxCollider(colliders, x, z, 0.8, 0.6, a + Math.PI / 2);
  }
  const altar = shadowed(new THREE.Mesh(new THREE.BoxGeometry(3.2, 0.7, 1.6), mat(0x8e879c)));
  altar.position.set(pad.x, terrain.heightAt(pad.x, pad.z) + 0.35, pad.z);
  g.add(altar);
  addBoxCollider(colliders, pad.x, pad.z, 1.7, 0.9);
  const crystal = new THREE.Mesh(new THREE.OctahedronGeometry(0.6, 0), glow(0xa0c4ff, 2));
  crystal.position.set(pad.x, terrain.heightAt(pad.x, pad.z) + 1.4, pad.z);
  g.add(crystal);
  scene.add(g);
}

function buildShrine(scene, terrain, colliders, pad) {
  const g = new THREE.Group();
  const y0 = terrain.heightAt(pad.x, pad.z) - 0.15;
  const red = mat(0xd9503f);
  // Torii gate.
  for (const sx of [-2.2, 2.2]) {
    const pillar = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(0.28, 0.32, 5, 8), red));
    pillar.position.set(sx, 2.5, 6);
    g.add(pillar);
    colliders.circles.push({ x: pad.x + sx, z: pad.z + 6, r: 0.4 });
  }
  const kasagi = shadowed(new THREE.Mesh(new THREE.BoxGeometry(7, 0.5, 0.6), red));
  kasagi.position.set(0, 5.3, 6);
  g.add(kasagi);
  const nuki = shadowed(new THREE.Mesh(new THREE.BoxGeometry(5.6, 0.35, 0.4), red));
  nuki.position.set(0, 4.3, 6);
  g.add(nuki);

  // Shrine hall.
  const hall = shadowed(new THREE.Mesh(new THREE.BoxGeometry(5, 3, 4), mat(0xf3e6d2)));
  hall.position.set(0, 1.9, -3);
  g.add(hall);
  const plinth = shadowed(new THREE.Mesh(new THREE.BoxGeometry(6, 0.5, 5), mat(PALETTE.stone)));
  plinth.position.set(0, 0.25, -3);
  g.add(plinth);
  const shape = new THREE.Shape();
  shape.moveTo(-3.6, 0); shape.lineTo(3.6, 0); shape.lineTo(0, 2.4); shape.closePath();
  const roofGeo = new THREE.ExtrudeGeometry(shape, { depth: 5.6, bevelEnabled: false });
  roofGeo.translate(0, 0, -2.8);
  const roof = shadowed(new THREE.Mesh(roofGeo, mat(0x4a4a6a)));
  roof.position.set(0, 3.3, -3);
  g.add(roof);
  const doorway = new THREE.Mesh(new THREE.BoxGeometry(1.4, 2, 0.12), glow(0xffb347, 1.2));
  doorway.position.set(0, 1.5, -0.95);
  g.add(doorway);
  addBoxCollider(colliders, pad.x, pad.z - 3, 3, 2.6);

  // Stone lanterns lining the approach.
  for (const sx of [-1.6, 1.6]) {
    for (const z of [2.5, 0]) {
      const post = shadowed(new THREE.Mesh(new THREE.BoxGeometry(0.4, 1.4, 0.4), mat(PALETTE.stone)));
      post.position.set(sx, 0.7, z);
      g.add(post);
      const box = new THREE.Mesh(new THREE.BoxGeometry(0.6, 0.5, 0.6), glow(0xffd28a, 1.5));
      box.position.set(sx, 1.6, z);
      g.add(box);
      const cap = shadowed(new THREE.Mesh(new THREE.ConeGeometry(0.55, 0.4, 4), mat(0x6e6a7a)));
      cap.position.set(sx, 2.05, z);
      cap.rotation.y = Math.PI / 4;
      g.add(cap);
      colliders.circles.push({ x: pad.x + sx, z: pad.z + z, r: 0.35 });
    }
  }
  // Blossom trees flank the hall.
  for (const sx of [-5, 5]) {
    const t = makeSmallTree(0xf29ca3, 1.5);
    t.position.set(sx, 0, -3);
    g.add(t);
    colliders.circles.push({ x: pad.x + sx, z: pad.z - 3, r: 0.5 });
  }
  const light = new THREE.PointLight(0xffb060, 10, 18, 2);
  light.position.set(0, 2.5, 0.5);
  g.add(light);

  g.position.set(pad.x, y0, pad.z);
  scene.add(g);
}

function buildDock(scene, terrain, colliders, pad, updaters) {
  const g = new THREE.Group();
  // Planks run from the pad toward open water (away from the island centre).
  const dir = new THREE.Vector2(pad.x, pad.z).normalize();
  const len = 22;
  const rot = Math.atan2(dir.x, dir.y);
  const cx = pad.x + dir.x * (8 + len * 0.5);
  const cz = pad.z + dir.y * (8 + len * 0.5);
  const deck = shadowed(new THREE.Mesh(new THREE.BoxGeometry(3.2, 0.3, len), mat(PALETTE.wood)));
  deck.position.set(cx, WATER_Y + 1.55, cz);
  deck.rotation.y = rot;
  g.add(deck);
  for (let i = 0; i <= 5; i++) {
    for (const s of [-1.4, 1.4]) {
      const along = -len / 2 + (i * len) / 5;
      const x = cx + dir.x * along + -dir.y * s;
      const z = cz + dir.y * along + dir.x * s;
      const post = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(0.16, 0.18, 3.4, 6), mat(0x7a5636)));
      post.position.set(x, WATER_Y + 0.6, z);
      g.add(post);
    }
  }
  // A little sailboat moored alongside, bobbing.
  const boat = new THREE.Group();
  const hullShape = new THREE.Shape();
  hullShape.moveTo(-2.4, 0); hullShape.lineTo(-1, 1); hullShape.lineTo(1.2, 1);
  hullShape.lineTo(2.6, 0); hullShape.lineTo(1.2, -1); hullShape.lineTo(-1, -1); hullShape.closePath();
  const hullGeo = new THREE.ExtrudeGeometry(hullShape, { depth: 0.9, bevelEnabled: false });
  hullGeo.rotateX(Math.PI / 2);
  const hull = shadowed(new THREE.Mesh(hullGeo, mat(0xe0574f)));
  hull.position.y = 0.9;
  boat.add(hull);
  const deckB = new THREE.Mesh(new THREE.BoxGeometry(3.2, 0.1, 1.6), mat(PALETTE.wood));
  deckB.position.y = 0.9;
  boat.add(deckB);
  const mast = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(0.07, 0.09, 4.5, 6), mat(0x7a5636)));
  mast.position.set(0.2, 3.1, 0);
  boat.add(mast);
  const sailShape = new THREE.Shape();
  sailShape.moveTo(0, 0); sailShape.lineTo(2.2, 0); sailShape.lineTo(0, 3.6); sailShape.closePath();
  const sail = shadowed(new THREE.Mesh(new THREE.ShapeGeometry(sailShape), mat(0xfaf6ee, { side: THREE.DoubleSide })));
  sail.position.set(0.3, 1.4, 0);
  sail.rotation.y = -Math.PI / 2;
  boat.add(sail);
  const bx = cx + -dir.y * 3.6 + dir.x * 2;
  const bz = cz + dir.x * 3.6 + dir.y * 2;
  boat.position.set(bx, WATER_Y - 0.5, bz);
  boat.rotation.y = rot + Math.PI / 2;
  g.add(boat);
  updaters.push((dt, t) => {
    boat.position.y = WATER_Y - 0.5 + Math.sin(t * 1.1) * 0.12;
    boat.rotation.z = Math.sin(t * 0.8) * 0.04;
  });
  colliders.circles.push({ x: bx, z: bz, r: 2.2 });

  // Crates on the beach end.
  const crate = shadowed(new THREE.Mesh(new THREE.BoxGeometry(1.1, 1.1, 1.1), mat(0xc79a62)));
  crate.position.set(pad.x - dir.y * 3, pad.y + 0.55, pad.z + dir.x * 3);
  crate.rotation.y = 0.4;
  g.add(crate);
  addBoxCollider(colliders, crate.position.x, crate.position.z, 0.6, 0.6, 0.4);

  // Walkable: the deck is a raised surface the player can stand on.
  colliders.platforms.push({ x: cx, z: cz, hw: 1.6, hd: len / 2, rot, y: WATER_Y + 1.7 });
  scene.add(g);
  return g;
}

// --- Sky things ------------------------------------------------------------------

function buildFloatingIslands(scene, rng, updaters) {
  const specs = [
    { x: 0, y: 62, z: -95, r: 9 },
    { x: -125, y: 50, z: 30, r: 7 },
    { x: 95, y: 72, z: 120, r: 11 },
    { x: 165, y: 56, z: -70, r: 6 },
    { x: -80, y: 78, z: -175, r: 8 },
  ];
  specs.forEach((s, i) => {
    const g = new THREE.Group();
    const rock = shadowed(new THREE.Mesh(new THREE.ConeGeometry(s.r, s.r * 1.6, 7), mat(0x9a95a8)));
    rock.rotation.x = Math.PI;
    rock.position.y = -s.r * 0.8;
    g.add(rock);
    const top = shadowed(new THREE.Mesh(new THREE.CylinderGeometry(s.r * 1.02, s.r * 0.92, 1.4, 7), mat(0x7cc48c)));
    top.position.y = 0.7;
    g.add(top);
    const nTrees = 2 + Math.floor(rng() * 3);
    for (let k = 0; k < nTrees; k++) {
      const a = rng() * Math.PI * 2;
      const rr = rng() * s.r * 0.55;
      const t = makeSmallTree(pick(rng, PALETTE.broadleaf), 0.8 + rng() * 0.6);
      t.position.set(Math.cos(a) * rr, 1.3, Math.sin(a) * rr);
      g.add(t);
    }
    if (i % 2 === 0) {
      const hut = makeHouse(mulberry32(i + 11), 2.6, 2.4, 2, pick(rng, PALETTE.walls), pick(rng, PALETTE.roofs));
      hut.position.set(0, 1.35, 0);
      hut.rotation.y = rng() * Math.PI * 2;
      g.add(hut);
    }
    g.position.set(s.x, s.y, s.z);
    scene.add(g);
    const phase = rng() * Math.PI * 2;
    updaters.push((dt, t) => {
      g.position.y = s.y + Math.sin(t * 0.35 + phase) * 1.6;
      g.rotation.y += dt * 0.03;
    });
  });
}

function buildClouds(scene, rng, updaters) {
  const cloudMat = new THREE.MeshStandardMaterial({ color: 0xffffff, flatShading: true, roughness: 1, transparent: true, opacity: 0.92 });
  const group = new THREE.Group();
  const clouds = [];
  for (let i = 0; i < 18; i++) {
    const c = new THREE.Group();
    const puffs = 3 + Math.floor(rng() * 3);
    for (let k = 0; k < puffs; k++) {
      const s = 5 + rng() * 7;
      const puff = new THREE.Mesh(new THREE.IcosahedronGeometry(s, 0), cloudMat);
      puff.position.set((k - puffs / 2) * s * 0.9, (rng() - 0.5) * 2, (rng() - 0.5) * 5);
      puff.scale.set(1, 0.55, 0.9);
      puff.castShadow = true;
      c.add(puff);
    }
    const r = 60 + rng() * 300;
    const a = rng() * Math.PI * 2;
    c.position.set(Math.cos(a) * r, 95 + rng() * 40, Math.sin(a) * r);
    group.add(c);
    clouds.push({ c, speed: 1 + rng() * 1.5 });
  }
  scene.add(group);
  updaters.push((dt) => {
    for (const { c, speed } of clouds) {
      c.position.x += dt * speed;
      if (c.position.x > 500) c.position.x = -500;
    }
  });
}

function buildAirship(scene, mixers) {
  const ship = new THREE.Group();
  const balloon = shadowed(new THREE.Mesh(new THREE.SphereGeometry(4, 10, 8), mat(0xf6b26b)));
  balloon.scale.set(1.9, 1, 1);
  ship.add(balloon);
  const stripe = new THREE.Mesh(new THREE.SphereGeometry(4.02, 10, 8), mat(0xe0574f));
  stripe.scale.set(0.6, 1.0, 1.0);
  ship.add(stripe);
  const gondola = shadowed(new THREE.Mesh(new THREE.BoxGeometry(5, 1.6, 2), mat(PALETTE.wood)));
  gondola.position.y = -4.6;
  ship.add(gondola);
  for (const sx of [-1.5, 1.5]) {
    const rope = new THREE.Mesh(new THREE.CylinderGeometry(0.05, 0.05, 1.6, 4), mat(0x3f3a4a));
    rope.position.set(sx, -3.4, 0.8);
    ship.add(rope);
    const rope2 = rope.clone();
    rope2.position.z = -0.8;
    ship.add(rope2);
  }
  const fin = shadowed(new THREE.Mesh(new THREE.BoxGeometry(2, 3, 0.15), mat(0xe0574f)));
  fin.position.set(-6.6, 0.8, 0);
  ship.add(fin);
  const finH = shadowed(new THREE.Mesh(new THREE.BoxGeometry(2, 0.15, 3), mat(0xe0574f)));
  finH.position.set(-6.6, 0, 0);
  ship.add(finH);
  const lantern = new THREE.Mesh(new THREE.SphereGeometry(0.35, 6, 6), glow(0xffd28a, 2));
  lantern.position.set(2.7, -3.6, 0);
  ship.add(lantern);
  scene.add(ship);

  // Closed loop around the island, sampled into position + heading keyframes.
  const pts = [
    [190, 46, 0], [130, 58, 150], [-30, 52, 200], [-175, 62, 105],
    [-200, 48, -60], [-95, 58, -190], [60, 50, -185], [175, 54, -95],
  ].map(([x, y, z]) => new THREE.Vector3(x, y, z));
  const curve = new THREE.CatmullRomCurve3(pts, true, 'centripetal');
  const N = 240;
  const duration = 150;
  const times = [];
  const posValues = [];
  const rotValues = [];
  const q = new THREE.Quaternion();
  const m = new THREE.Matrix4();
  const up = new THREE.Vector3(0, 1, 0);
  for (let i = 0; i <= N; i++) {
    const u = (i % N) / N;
    const p = curve.getPointAt(u);
    const tan = curve.getTangentAt(u);
    times.push((i / N) * duration);
    posValues.push(p.x, p.y, p.z);
    // Ship's nose points +x, so look along the tangent.
    m.lookAt(new THREE.Vector3(0, 0, 0), tan.clone().negate(), up);
    q.setFromRotationMatrix(m);
    q.multiply(new THREE.Quaternion().setFromAxisAngle(up, -Math.PI / 2));
    rotValues.push(q.x, q.y, q.z, q.w);
  }
  const clip = new THREE.AnimationClip('patrol', duration, [
    new THREE.VectorKeyframeTrack('.position', times, posValues),
    new THREE.QuaternionKeyframeTrack('.quaternion', times, rotValues),
  ]);
  const mixer = new THREE.AnimationMixer(ship);
  mixer.clipAction(clip).play();
  mixers.push(mixer);
}

function buildBirds(scene, updaters, pad) {
  const flock = new THREE.Group();
  const wingGeo = new THREE.PlaneGeometry(0.9, 0.35);
  wingGeo.translate(0.45, 0, 0);
  const birdMat = new THREE.MeshStandardMaterial({ color: 0x3f3a4a, side: THREE.DoubleSide, roughness: 1 });
  const bodyGeo = new THREE.ConeGeometry(0.16, 0.9, 4);
  bodyGeo.rotateX(Math.PI / 2);
  const birds = [];
  for (let i = 0; i < 12; i++) {
    const b = new THREE.Group();
    const l = new THREE.Mesh(wingGeo, birdMat);
    const r = new THREE.Mesh(wingGeo, birdMat);
    r.rotation.y = Math.PI;
    b.add(l, r, new THREE.Mesh(bodyGeo, birdMat));
    flock.add(b);
    birds.push({ b, l, r, phase: Math.random() * Math.PI * 2, r0: 22 + Math.random() * 18, h: 24 + Math.random() * 10, speed: 0.25 + Math.random() * 0.15 });
  }
  flock.position.set(pad.x, 0, pad.z);
  scene.add(flock);
  updaters.push((dt, t) => {
    for (const bd of birds) {
      const a = t * bd.speed + bd.phase;
      bd.b.position.set(Math.cos(a) * bd.r0, pad.y + bd.h + Math.sin(t * 0.7 + bd.phase) * 1.5, Math.sin(a) * bd.r0);
      bd.b.rotation.y = -a;
      const flap = Math.sin(t * 9 + bd.phase) * 0.7;
      bd.l.rotation.z = flap;
      bd.r.rotation.z = -flap;
    }
  });
}

