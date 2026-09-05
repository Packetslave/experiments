import * as THREE from 'three';
import { PointerLockControls } from 'three/addons/controls/PointerLockControls.js';
import { WATER_Y } from './terrain.js';
import { clamp } from './noise.js';

// First-person walker: pointer-lock mouse look, WASD, sprint, jump, gravity,
// terrain following, wading/swimming, and simple push-out collision against
// the world's box and circle colliders.

const EYE = 1.7;
const RADIUS = 0.45;
const WALK = 6;
const SPRINT = 11;
const JUMP = 7.5;
const GRAVITY = 22;
const STEP_LIMIT = 1.1;   // max instantaneous rise before a slope counts as a wall
const SWIM_FEET = WATER_Y - 1.25;

export class Player {
  constructor(camera, domElement, terrain, colliders) {
    this.camera = camera;
    this.terrain = terrain;
    this.colliders = colliders;
    this.controls = new PointerLockControls(camera, domElement);
    this.controls.pointerSpeed = 0.9;
    this.velocity = new THREE.Vector3();
    this.feet = new THREE.Vector3();
    this.onGround = false;
    this.keys = new Set();
    this.#buildGrid();

    window.addEventListener('keydown', (e) => {
      if (e.repeat) return;
      this.keys.add(e.code);
      if (['Space', 'KeyW', 'KeyA', 'KeyS', 'KeyD'].includes(e.code) && this.controls.isLocked) e.preventDefault();
    });
    window.addEventListener('keyup', (e) => this.keys.delete(e.code));
    window.addEventListener('blur', () => this.keys.clear());
  }

  get isLocked() {
    return this.controls.isLocked;
  }

  lock() {
    this.controls.lock();
  }

  spawn(x, z, yawDeg = 0) {
    this.feet.set(x, this.#groundAt(x, z), z);
    this.velocity.set(0, 0, 0);
    this.camera.position.set(x, this.feet.y + EYE, z);
    this.camera.rotation.set(0, THREE.MathUtils.degToRad(yawDeg), 0, 'YXZ');
  }

  // Spatial hash for the circle colliders (a few thousand trees).
  #buildGrid() {
    this.cellSize = 8;
    this.grid = new Map();
    for (const c of this.colliders.circles) {
      const key = `${Math.floor(c.x / this.cellSize)},${Math.floor(c.z / this.cellSize)}`;
      if (!this.grid.has(key)) this.grid.set(key, []);
      this.grid.get(key).push(c);
    }
  }

  #groundAt(x, z) {
    let g = Math.max(this.terrain.heightAt(x, z), SWIM_FEET);
    for (const p of this.colliders.platforms) {
      const dx = x - p.x, dz = z - p.z;
      const c = Math.cos(p.rot), s = Math.sin(p.rot);
      const lx = dx * c - dz * s;
      const lz = dx * s + dz * c;
      if (Math.abs(lx) <= p.hw && Math.abs(lz) <= p.hd && p.y >= g - 0.5) g = Math.max(g, p.y);
    }
    return g;
  }

  #inWater(x, z) {
    return this.terrain.heightAt(x, z) < WATER_Y - 0.5 && this.#groundAt(x, z) < WATER_Y;
  }

  #resolveCollisions(pos) {
    // Boxes (houses etc.), rotated about y.
    for (const b of this.colliders.boxes) {
      const dx = pos.x - b.x, dz = pos.z - b.z;
      if (Math.abs(dx) > b.hw + b.hd + 2 || Math.abs(dz) > b.hw + b.hd + 2) continue;
      const c = Math.cos(b.rot), s = Math.sin(b.rot);
      const lx = dx * c - dz * s;
      const lz = dx * s + dz * c;
      const cx = clamp(lx, -b.hw, b.hw);
      const cz = clamp(lz, -b.hd, b.hd);
      let px = lx - cx, pz = lz - cz;
      let d = Math.hypot(px, pz);
      if (d >= RADIUS) continue;
      if (d < 1e-6) {
        // Inside the box: push out along the nearest face.
        const ex = b.hw - Math.abs(lx), ez = b.hd - Math.abs(lz);
        if (ex < ez) { px = Math.sign(lx) || 1; pz = 0; d = 0; cx = Math.sign(lx) * b.hw; }
        else { pz = Math.sign(lz) || 1; px = 0; d = 0; cz = Math.sign(lz) * b.hd; }
        const len = Math.hypot(px, pz);
        px /= len; pz /= len;
        const nlx = cx + px * RADIUS, nlz = cz + pz * RADIUS;
        pos.x = b.x + nlx * c + nlz * s;
        pos.z = b.z - nlx * s + nlz * c;
        continue;
      }
      const push = (RADIUS - d) / d;
      const nlx = lx + px * push, nlz = lz + pz * push;
      pos.x = b.x + nlx * c + nlz * s;
      pos.z = b.z - nlx * s + nlz * c;
    }
    // Circles (trunks, posts, rocks) via the grid.
    const gx = Math.floor(pos.x / this.cellSize), gz = Math.floor(pos.z / this.cellSize);
    for (let ix = gx - 1; ix <= gx + 1; ix++) {
      for (let iz = gz - 1; iz <= gz + 1; iz++) {
        const cell = this.grid.get(`${ix},${iz}`);
        if (!cell) continue;
        for (const c of cell) {
          const dx = pos.x - c.x, dz = pos.z - c.z;
          const d = Math.hypot(dx, dz);
          const min = c.r + RADIUS;
          if (d < min && d > 1e-6) {
            pos.x = c.x + (dx / d) * min;
            pos.z = c.z + (dz / d) * min;
          }
        }
      }
    }
  }

  update(dt) {
    dt = Math.min(dt, 0.05);
    const k = this.keys;
    const locked = this.controls.isLocked;

    // Desired horizontal direction in world space from camera yaw.
    const fwd = new THREE.Vector3();
    this.camera.getWorldDirection(fwd);
    fwd.y = 0;
    fwd.normalize();
    const right = new THREE.Vector3().crossVectors(fwd, this.camera.up).normalize();
    const wish = new THREE.Vector3();
    if (locked) {
      if (k.has('KeyW') || k.has('ArrowUp')) wish.add(fwd);
      if (k.has('KeyS') || k.has('ArrowDown')) wish.sub(fwd);
      if (k.has('KeyD') || k.has('ArrowRight')) wish.add(right);
      if (k.has('KeyA') || k.has('ArrowLeft')) wish.sub(right);
    }
    const swimming = this.#inWater(this.feet.x, this.feet.z);
    let speed = k.has('ShiftLeft') || k.has('ShiftRight') ? SPRINT : WALK;
    if (swimming) speed *= 0.45;
    if (wish.lengthSq() > 0) wish.normalize().multiplyScalar(speed);

    const accel = this.onGround ? 12 : 3;
    const blend = 1 - Math.exp(-accel * dt);
    this.velocity.x += (wish.x - this.velocity.x) * blend;
    this.velocity.z += (wish.z - this.velocity.z) * blend;

    if (locked && k.has('Space') && this.onGround && !swimming) {
      this.velocity.y = JUMP;
      this.onGround = false;
    }
    this.velocity.y -= GRAVITY * dt;
    if (swimming) this.velocity.y = Math.max(this.velocity.y, -2);

    // Horizontal move with a wall/step test, sliding along each axis.
    const half = this.terrain.size * 0.5 - 4;
    const next = this.feet.clone();
    const tryMove = (dx, dz) => {
      const nx = clamp(next.x + dx, -half, half);
      const nz = clamp(next.z + dz, -half, half);
      const g = this.#groundAt(nx, nz);
      if (g - this.feet.y > STEP_LIMIT) return false;
      next.x = nx;
      next.z = nz;
      return true;
    };
    if (!tryMove(this.velocity.x * dt, this.velocity.z * dt)) {
      if (!tryMove(this.velocity.x * dt, 0)) this.velocity.x = 0;
      if (!tryMove(0, this.velocity.z * dt)) this.velocity.z = 0;
    }
    this.#resolveCollisions(next);
    this.feet.x = next.x;
    this.feet.z = next.z;

    // Vertical.
    this.feet.y += this.velocity.y * dt;
    const ground = this.#groundAt(this.feet.x, this.feet.z);
    if (this.feet.y <= ground) {
      this.feet.y = ground;
      this.velocity.y = 0;
      this.onGround = true;
    } else {
      this.onGround = this.feet.y - ground < 0.05;
    }

    // Head bob while walking on land.
    const hSpeed = Math.hypot(this.velocity.x, this.velocity.z);
    this.bobPhase = (this.bobPhase || 0) + dt * hSpeed * 1.6;
    const bob = this.onGround && !swimming ? Math.sin(this.bobPhase) * 0.035 * Math.min(1, hSpeed / WALK) : 0;
    this.camera.position.set(this.feet.x, this.feet.y + EYE + bob, this.feet.z);
    this.swimming = swimming;
  }
}
