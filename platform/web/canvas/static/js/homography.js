const EPSILON = 1e-12;

// Homography is a 2D projective transform stored as a flat 3x3 row-major
// array:
//   [h0 h1 h2]   x' = (h0*x + h1*y + h2) / w
//   [h3 h4 h5]   y' = (h3*x + h4*y + h5) / w
//   [h6 h7 h8]   w  =  h6*x + h7*y + h8
//
// homographyFromQuad builds the projective transform that maps the unit
// square (0,0)(1,0)(1,1)(0,1) onto the four corners c[0..3], given in that
// same order (top-left, top-right, bottom-right, bottom-left in destination
// space). Ported from math2d.HomographyFromQuad (Heckbert's closed-form
// square->quad solution): no 8x8 solve, the special source (the unit
// square) lets the system decouple.
//
// Returns null when the corners are degenerate (any three collinear),
// which would make the map non-invertible.
export function homographyFromQuad(c) {
    const p0 = c[0];
    const p1 = c[1];
    const p2 = c[2];
    const p3 = c[3];

    // s is the "diagonal defect": p0 - p1 + p2 - p3. It is exactly zero when
    // the quad is a parallelogram (the two diagonals share a midpoint),
    // which is the same as "no perspective". s != 0 means the map needs a
    // bottom row.
    const sX = p0.x - p1.x + p2.x - p3.x;
    const sY = p0.y - p1.y + p2.y - p3.y;

    if (Math.abs(sX) <= EPSILON && Math.abs(sY) <= EPSILON) {
        // Affine fast path: w is constant (h6 = h7 = 0), no division needed
        // later.
        const h = [
            p1.x - p0.x, p3.x - p0.x, p0.x,
            p1.y - p0.y, p3.y - p0.y, p0.y,
            0, 0, 1,
        ];
        // A collinear parallelogram has zero area -> not invertible.
        if (Math.abs(determinant(h)) <= EPSILON) {
            return null;
        }
        return h;
    }

    // Projective path. The perspective coefficients g, h are the unique
    // solution of the 2x2 system g*d1 + h*d2 = s, with d1 = p1-p2,
    // d2 = p3-p2.
    const d1X = p1.x - p2.x;
    const d1Y = p1.y - p2.y;
    const d2X = p3.x - p2.x;
    const d2Y = p3.y - p2.y;

    const den = d1X * d2Y - d2X * d1Y; // cross(d1, d2)
    if (Math.abs(den) <= EPSILON) {
        return null; // p1, p2, p3 collinear -> no unique solution
    }

    const invDen = 1 / den;
    const g = (sX * d2Y - d2X * sY) * invDen; // Cramer's rule
    const h6 = (d1X * sY - sX * d1Y) * invDen;

    // Top two rows are then forced so that (1,0)->p1 and (0,1)->p3; the
    // bottom row (g, h6, 1) is what bends (1,1) onto p2 with the right
    // perspective.
    const h = [
        p1.x - p0.x + g * p1.x, p3.x - p0.x + h6 * p3.x, p0.x,
        p1.y - p0.y + g * p1.y, p3.y - p0.y + h6 * p3.y, p0.y,
        g, h6, 1,
    ];
    if (Math.abs(determinant(h)) <= EPSILON) {
        return null;
    }
    return h;
}

// applyHomography maps the point (x, y) through the projective transform h,
// performing the perspective divide.
export function applyHomography(h, x, y) {
    const w = h[6] * x + h[7] * y + h[8];
    return {
        x: (h[0] * x + h[1] * y + h[2]) / w,
        y: (h[3] * x + h[4] * y + h[5]) / w,
    };
}

function determinant(h) {
    return h[0] * (h[4] * h[8] - h[5] * h[7]) -
        h[1] * (h[3] * h[8] - h[5] * h[6]) +
        h[2] * (h[3] * h[7] - h[4] * h[6]);
}
