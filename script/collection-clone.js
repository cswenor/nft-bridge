import algosdk, { mnemonicToSecretKey } from "algosdk";
import dotenv from "dotenv";
import figlet from "figlet";
import { getAlgorandClients, signSendAndConfirm } from "./utils.js";
import fs from "fs";
import cliProgress from "cli-progress";

dotenv.config();

console.log(figlet.textSync("COLLECTION CLONE", { horizontalLayout: "full" }));
console.log(
  "Clones a collection from one address to another for testing purposes"
);

if (process.argv.length !== 4) {
  console.log("");
  console.log("Usage: node script/collection-clone.js COLLECTION_ADDR");
  console.log("");
  process.exit(1); // error invalid args
}

const [, , COLLECTION_NAME, COLLECTION_ADDR] = process.argv;

const isALGOAddress = (addr) => true; // TODO: validate ALGO address
if (!isALGOAddress(COLLECTION_ADDR)) {
  process.exi(2); // error invalid address
}

const { ADDR, MN, MN2, MN3, NODE, BRIDGE_ADDR, COLL_NODE } = process.env;

const bridgeAddress = BRIDGE_ADDR || "";

const addrTo = ADDR;

const mnemonic = MN || "";

const { addr: address, sk } = mnemonicToSecretKey(mnemonic);
const { addr: address2, sk: sk2 } = mnemonicToSecretKey(MN2 || "");
const { addr: address3, sk: sk3 } = mnemonicToSecretKey(MN3 || "");

const [node] = NODE.split(":");
const [collNode] = COLL_NODE.split(":");

console.log("address", address);
console.log("address2", address2);
console.log("address3", address3);
console.log("bridgeAddress", bridgeAddress);

const [algodClient, indexerClient] = getAlgorandClients(node);
const [collAlgodClient, collIndexerClient] = getAlgorandClients(collNode);

const main = async () => {
  console.log("Clone!");
  const { assets } = await collIndexerClient
    .lookupAccountCreatedAssets(COLLECTION_ADDR)
    .do();
  // no need to try to clone whole collection

  const bar1 = new cliProgress.SingleBar(
    {},
    cliProgress.Presets.shades_classic
  );
  let i = 0;
  bar1.start(10, i);
  const clonedAssets = [];
  for (const collAsset of assets.slice(0, 10)) {
    if (collAsset.params.total !== 1) continue; // skip non-NFTs
    const txn = algosdk.makeAssetCreateTxnWithSuggestedParamsFromObject({
      from: address,
      assetName: collAsset.params.name,
      unitName: collAsset.params["unit-name"],
      total: collAsset.params.total,
      decimals: collAsset.params.decimals,
      assetURL: collAsset.params.url,
      manager: address,
      reserve: address,
      freeze: address,
      clawback: address,
      suggestedParams: await algodClient.getTransactionParams().do(),
    });
    const [txnId] = await signSendAndConfirm(algodClient, [txn], sk);
    const {
      transaction: { ["created-asset-index"]: assetId },
    } = await indexerClient.lookupTransactionByID(txnId).do();
    const { asset } = await indexerClient.lookupAssetByID(assetId).do();
    const newAsset = {
      ...asset,
      originAssetId: collAsset.index,
      originNode: collNode,
    };
    clonedAssets.push(newAsset);
    bar1.update(++i);
  }
  bar1.update(10);
  bar1.stop();
  console.log(
    `Cloned ${clonedAssets.length} assets to /collections/${COLLECTION_NAME}.json`
  );
  if (!fs.existsSync("./collections")) {
    fs.mkdirSync("./collections", { recursive: true });
  }
  fs.writeFileSync(
    `./collections/${COLLECTION_NAME}.json`,
    JSON.stringify(clonedAssets, null, 2)
  );
};

main();
