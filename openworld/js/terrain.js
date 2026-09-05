import * as THREE from 'three';
import { Noise2D, smoothstep, lerp, clamp } from './noise.js';

// The island. A heightfield sampled onto a PlaneGeometry, coloured per face
// (low-poly look), plus an exact height query that follows the mesh's own
// triangulation so the player never floats or sinks.

export const WATER_Y = 0;

export class Terrain {
  /**
   * @param {object} opts
   * @param {number} opts.size      world units on a side
   * @param {number} opts.segments  grid cells on a side
   * @param {number} opts.seed
   * @param {Array<{x:number,z:number,r:number,y?:number,minY?:number}>} opts.pads
   *        flat landing pads for points of interest; y is resolved from the
   *        natural terrain if omitted (clamped to minY)
   */
  constructor({ size = 520, segments = 260, seed = 7, pads = [] }) {
    this.size = size;
    this.segments = segments;
    this.cell = size / segments;
    this.noise = new Noise2D(seed);
    this.detail = new Noise2D(seed * 31 + 7);
    this.pads = pads;

    // Resolve pad heights against the untouched landscape first.
    for (const p of this.pads) {
      if (p.y === undefined) p.y = Math.max(p.minY ?? 3, this.#baseHeight(p.x, p.z));
    }

    this.#buildHeights();
    this.mesh = this.#buildMesh();
  }

  // --- Height function -----------------------------------------------------

  #baseHeight(x, z) {
    const half = this.size * 0.5;
    const d = Math.hypot(x, z) / half; // 0 at centre, 1 at the edge
    const n = this.noise.fbm(x * 0.0055, z * 0.0055, 5, 2.1, 0.5);
    const ridge = 1 - Math.abs(this.noise.fbm(x * 0.011 + 40, z * 0.011 - 17, 3, 2, 0.5));
    let h = 9 + n * 17 + ridge * 9 * Math.max(0, n);

    // A guaranteed mountain to climb, with a snowy top.
    const mdx = x + 40, mdz = z + 70;
    h += 24 * Math.exp(-(mdx * mdx + mdz * mdz) / (2 * 48 * 48));

    // Fall away into the sea toward the edges so it reads as an island.
    const mask = smoothstep(0.56, 0.98, d);
    h = h * (1 - mask) - mask * 22;

    // Small-scale bumps for the low-poly facets to catch light on.
    h += this.detail.noise(x * 0.06, z * 0.06) * 0.6;
    return h;
  }

  #height(x, z) {
    let h = this.#baseHeight(x, z);
    for (const p of this.pads) {
      const d = Math.hypot(x - p.x, z - p.z);
      if (d < p.r * 1.6) {
        const t = 1 - smoothstep(p.r * 0.55, p.r * 1.6, d);
        h = lerp(h, p.y, t);
      }
    }
    return h;
  }

  #buildHeights() {
    const n = this.segments + 1;
    const half = this.size * 0.5;
    const heights = new Float32Array(n * n);
    for (let iz = 0; iz < n; iz++) {
      const z = -half + iz * this.cell;
      for (let ix = 0; ix < n; ix++) {
        const x = -half + ix * this.cell;
        heights[iz * n + ix] = this.#height(x, z);
      }
    }
    this.heights = heights;
  }

  /** Exact height of the rendered surface at (x, z). */
  heightAt(x, z) {
    const n = this.segments + 1;
    const half = this.size * 0.5;
    const fx = clamp((x + half) / this.cell, 0, this.segments - 1e-6);
    const fz = clamp((z + half) / this.cell, 0, this.segments - 1e-6);
    const ix = Math.floor(fx);
    const iz = Math.floor(fz);
    const tx = fx - ix;
    const tz = fz - iz;
    const H = this.heights;
    const ha = H[iz * n + ix];           // (ix,   iz)
    const hb = H[(iz + 1) * n + ix];     // (ix,   iz+1)
    const hc = H[(iz + 1) * n + ix + 1]; // (ix+1, iz+1)
    const hd = H[iz * n + ix + 1];       // (ix+1, iz)
    // PlaneGeometry splits each cell along the b-d diagonal.
    if (tx + tz <= 1) return ha + (hd - ha) * tx + (hb - ha) * tz;
    return hc + (hb - hc) * (1 - tx) + (hd - hc) * (1 - tz);
  }

  /** Approximate surface normal (for slope tests when scattering). */
  normalAt(x, z, out = new THREE.Vector3()) {
    const e = this.cell;
    const hl = this.heightAt(x - e, z), hr = this.heightAt(x + e, z);
    const hd = this.heightAt(x, z - e), hu = this.heightAt(x, z + e);
    return out.set(hl - hr, 2 * e, hd - hu).normalize();
  }

  // --- Mesh ----------------------------------------------------------------

  #buildMesh() {
    const n = this.segments + 1;
    let geo = new THREE.PlaneGeometry(this.size, this.size, this.segments, this.segments);
    geo.rotateX(-Math.PI / 2);
    const pos = geo.attributes.position;
    for (let i = 0; i < pos.count; i++) {
      const ix = i % n;
      const iz = Math.floor(i / n);
      pos.setY(i, this.heights[iz * n + ix]);
    }
    geo = geo.toNonIndexed();
    geo.computeVertexNormals();

    // One colour per triangle.
    const p = geo.attributes.position;
    const nor = geo.attributes.normal;
    const colors = new Float32Array(p.count * 3);
    const c = new THREE.Color();
    const sand = new THREE.Color(0xeadfb4);
    const grassLo = new THREE.Color(0x8ed48a);
    const grassHi = new THREE.Color(0x5fae6c);
    const rock = new THREE.Color(0xa49fb0);
    const rockDark = new THREE.Color(0x857f94);
    const snow = new THREE.Color(0xf6f8fb);
    const seabed = new THREE.Color(0x9fbfae);
    const tmp = new THREE.Color();
    for (let f = 0; f < p.count; f += 3) {
      const h = (p.getY(f) + p.getY(f + 1) + p.getY(f + 2)) / 3;
      const ny = nor.getY(f);
      const cx = (p.getX(f) + p.getX(f + 1) + p.getX(f + 2)) / 3;
      const cz = (p.getZ(f) + p.getZ(f + 1) + p.getZ(f + 2)) / 3;
      const v = this.detail.noise(cx * 0.03 + 9, cz * 0.03 - 3); // colour jitter

      if (h < WATER_Y - 1.5) {
        c.copy(seabed).lerp(sand, smoothstep(-12, -1.5, h));
      } else if (h < WATER_Y + 0.9) {
        c.copy(sand);
      } else {
        const t = smoothstep(1, 24, h);
        c.copy(grassLo).lerp(grassHi, t);
        c.offsetHSL(v * 0.02, v * 0.05, v * 0.03);
        if (h > 20) c.lerp(snow, smoothstep(20, 27, h));
      }
      if (ny < 0.72 && h > WATER_Y - 1.5) {
        tmp.copy(rock).lerp(rockDark, clamp(0.5 - v, 0, 1));
        c.lerp(tmp, smoothstep(0.72, 0.5, ny));
      }
      for (let k = 0; k < 3; k++) {
        colors[(f + k) * 3] = c.r;
        colors[(f + k) * 3 + 1] = c.g;
        colors[(f + k) * 3 + 2] = c.b;
      }
    }
    geo.setAttribute('color', new THREE.BufferAttribute(colors, 3));

    const mat = new THREE.MeshStandardMaterial({
      vertexColors: true,
      flatShading: true,
      roughness: 0.95,
      metalness: 0,
    });
    const mesh = new THREE.Mesh(geo, mat);
    mesh.receiveShadow = true;
    mesh.castShadow = true;
    mesh.name = 'terrain';
    return mesh;
  }
}
