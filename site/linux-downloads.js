// The Linux packages can be published between static site deployments.
async function refreshLinuxDownloads(section) {
  const repo = "fclaude/fclaude-packages";
  const response = await fetch(`https://api.github.com/repos/${repo}/releases/latest`, {
    cache: "no-cache",
    credentials: "omit",
  });
  if (!response.ok) return;
  const release = await response.json();
  const tag = release.tag_name;
  if (release.draft || release.prerelease || !/^packages-\d{4}-\d{2}-\d{2}-\d+$/.test(tag) ||
      tag === section.dataset.tag) return;

  const files = new Set(release.assets.filter(asset => asset.state === "uploaded").map(asset => asset.name));
  const versions = [...files].flatMap(name => {
    const match = /^dinoscript_(\d+\.\d+\.\d+-\d+)_amd64\.deb$/.exec(name);
    return match ? [match[1]] : [];
  });
  if (versions.length !== 1) return;
  const [version] = versions;
  const base = `https://github.com/${repo}/releases`;
  const updates = [];
  for (const link of section.querySelectorAll(".downloads a, .download-meta a")) {
    const name = new URL(link.href).pathname.split("/").pop().replace(section.dataset.version, version);
    if (!files.has(name)) return;
    updates.push([link, `${base}/download/${tag}/${name}`]);
  }
  // Switch all five installers and their checksum links together.
  for (const [link, url] of updates) link.href = url;
  section.querySelector(".release-link").href = `${base}/tag/${tag}`;
  section.querySelector(".linux-version").textContent = version;
  section.dataset.tag = tag;
  section.dataset.version = version;
}

const linuxDownloads = document.querySelector(".linux-downloads");
if (linuxDownloads) {
  // The prebuilt links stay usable when GitHub is offline or rate-limits a visitor.
  refreshLinuxDownloads(linuxDownloads).catch(() => {});
}
