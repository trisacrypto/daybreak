import Alerts from '../modules/alerts.js';
import { clearFormErrors, markFormErrors } from '../modules/forms.js';

/**************************************************
 * Create Counterparty Handling
 **************************************************/

const createEntryForm = document.getElementById('createEntryForm');

if (createEntryForm) {
  const createEntryAlerts = new Alerts(document.getElementById('createEntryAlerts'), { dismissible: true, autoClose: true });
  createEntryForm.addEventListener('htmx:afterRequest', function(e) {
    clearFormErrors(createEntryForm);
    createEntryAlerts.clear();

    if (e.detail.successful) {
      createEntryForm.reset();
      createEntryAlerts.success('', 'Counterparty successfully created.', { dismissible: true, autoClose: true });
    } else {
      markFormErrors(createEntryForm, e, createEntryAlerts);
    }
  });

  // Reset state when the modal is closed.
  const createEntryModal = document.getElementById('createEntryModal');
  createEntryModal.addEventListener('hidden.bs.modal', function() {
    createEntryForm.reset();
    clearFormErrors(createEntryForm);
    createEntryAlerts.clear();
  });
}

/**************************************************
 * Create License Handling
 **************************************************/


const createLicenseForm = document.getElementById('createLicenseForm');

if (createLicenseForm) {
  const createLicenseAlerts = new Alerts(document.getElementById('createLicenseAlerts'), { dismissible: true, autoClose: true });
  createLicenseForm.addEventListener('htmx:afterRequest', function(e) {
    clearFormErrors(createLicenseForm);
    createLicenseAlerts.clear();

    if (e.detail.successful) {
      createLicenseForm.reset();
      createLicenseAlerts.success('', 'License key successfully issued.', { dismissible: true, autoClose: true });
    } else {
      markFormErrors(createLicenseForm, e, createLicenseAlerts);
    }
  });

  // Reset state when the modal is closed.
  const createLicenseModal = document.getElementById('createLicenseModal');
  createLicenseModal.addEventListener('hidden.bs.modal', function() {
    createLicenseForm.reset();
    clearFormErrors(createLicenseForm);
    createLicenseAlerts.clear();
  });
}

/**************************************************
 * Countries Choropleth Chart
 **************************************************/

const fetchJSON = (url) => {
  return fetch(url).then((response) => {
    if (!response.ok) {
      throw new Error(`${url} responded with ${response.status}`);
    }
    return response.json();
  });
}

// ColorBrewer 9-class Blues, light to dark (the same ramp as the built-in 'blues').
const BLUES = [
  [247, 251, 255], [222, 235, 247], [198, 219, 239], [158, 202, 225], [107, 174, 214],
  [66, 146, 198], [33, 113, 181], [8, 81, 156], [8, 48, 107],
];

// Exponent of the power law: 1 is linear, values below 1 spread out the low end
// of the scale (0.5 is a square root scale).
const POWER_EXPONENT = 0.25;

// Interpolates the blues color ramp using a power law. The color scale passes in
// the value normalized linearly to [0, 1]; the color is picked at t^POWER_EXPONENT.
const powerBlues = (t) => {
  const p = Math.pow(Math.min(Math.max(t, 0), 1), POWER_EXPONENT) * (BLUES.length - 1);
  const i = Math.min(Math.floor(p), BLUES.length - 2);
  const [r, g, b] = BLUES[i].map((c, k) => Math.round(c + (BLUES[i + 1][k] - c) * (p - i)));
  return `rgb(${r}, ${g}, ${b})`;
};

const globalVASPsChart = () => {
  Promise.all([fetchJSON('/v1/countries'), fetchJSON('/static/js/geojson/countries-110m.json')])
    .then(([counts, topo]) => {
      const countries = topojson.feature(topo, topo.objects.countries).features.filter((country) => country.id !== '010');
      const data = countries.map((country) => {
        return {
          feature: country,
          value: counts.find((c) => c.code === country.id)?.count || 0,
        }
      });

      const labels = countries.map((country) => {
        if (country.properties.name == 'United States of America') {
          return 'United States';
        }
        return country.properties.name;
      });

      const chart = new Chart(document.getElementById('globalVASPsChart').getContext('2d'), {
        type: 'choropleth',
        data: {
          labels: labels,
          datasets: [
            {
              label: 'Global VASPs',
              data: data,
              outline: countries,
            }
          ]
        },
        options: {
          showOutline: false,
          showGraticule: false,
          plugins: {
            legend: {
              display: false,
            },
            // The Dashkit theme defaults tooltips to mode 'index' with intersect
            // false, which picks the country by x position instead of the country
            // under the cursor; a map needs to hit test the country shape.
            tooltip: {
              mode: 'nearest',
              intersect: true,
            },
          },
          scales: {
            projection: {
              axis: 'x',
              projection: 'equirectangular',
            },
            color: {
              axis: 'x',
              interpolate: powerBlues,
              legend: {
                position: 'bottom-right',
                align: 'right'
              }
            },
          },
        },
      });
    })
    .catch((err) => {
      console.error(err);
    });
}

document.addEventListener("DOMContentLoaded", () => {
  globalVASPsChart();
});