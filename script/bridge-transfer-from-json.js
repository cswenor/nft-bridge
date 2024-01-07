import algosdk, {
  mnemonicToSecretKey,
  signTransaction,
  makePaymentTxnWithSuggestedParamsFromObject,
  makeAssetTransferTxnWithSuggestedParamsFromObject,
  waitForConfirmation,
} from "algosdk";
import dotenv from "dotenv";
import figlet from "figlet";
import { algoWalkAssetIds, alienTourismAssetIds } from "./wl.js";
import { signSendAndConfirm } from "./utils.js";
import { getAlgorandClients } from "./utils.js";
import fs from "fs";

dotenv.config();

console.log(figlet.textSync("NFT BRIDGE", { horizontalLayout: "full" }));

const { ADDR, MN, MN2, MN3, NODE, BRIDGE_ADDR } = process.env;

if (process.argv.length !== 3) process.exit(1);

const [, , infile] = process.argv;

const data = JSON.parse(fs.readFileSync(infile, "utf8"));

const bridgeAddress = BRIDGE_ADDR || "";

const addrTo = ADDR;

const mnemonic = MN || "";

const { addr: address, sk } = mnemonicToSecretKey(mnemonic);
const { addr: address2, sk: sk2 } = mnemonicToSecretKey(MN2 || "");
const { addr: address3, sk: sk3 } = mnemonicToSecretKey(MN3 || "");

const [node] = NODE.split(":");

console.log("address", address);
console.log("address2", address2);
console.log("address3", address3);
console.log("bridgeAddress", bridgeAddress);

const [algodClient, indexerClient] = getAlgorandClients(node);

const makeBridgeTransferTxn = async (asset, addrTo) => {
  const transactionFee = 0.101;
  const serviceFee = 0.099;
  const totalFee = transactionFee + serviceFee;
  const req = {
    assetId: asset.index,
    to: addrTo,
    amount: 1,
  };
  return makeAssetTransferTxnWithSuggestedParamsFromObject({
    assetIndex: asset.index,
    from: address,
    to: bridgeAddress,
    amount: 1,
    note: new Uint8Array(Buffer.from(JSON.stringify(req))),
    suggestedParams: await algodClient.getTransactionParams().do(),
  });
};

const main = async () => {
  console.log("Do the thing!");
  const { assets: myAssets } = await indexerClient
    .lookupAccountAssets(address)
    .do();
  const { assets: bridgeAssets } = await indexerClient
    .lookupAccountAssets(bridgeAddress)
    .do();
  for (const asset of data) {
    const myAsset = myAssets.find(({ ["asset-id"]: assetId }) => assetId === asset.index);
    if (!myAsset) {
      console.log(`Asset ${asset.index} not found in my account`);
      continue;
    }
    if(myAsset.amount === 0) {
      console.log(`Asset ${asset.index} amount is 0`);
      continue;
    }
    const bridgeAsset = bridgeAssets.find(({ ["asset-id"]: assetId }) => assetId === asset.index);
    if (!bridgeAsset) {
      console.log(`Asset ${asset.index} not found in bridge account`);
      continue;
    }
    if(bridgeAsset.amount === 1) {
      console.log(`Asset ${asset.index} already transferred`);
      continue;
    }
    await signSendAndConfirm(
      algodClient,
      [await makeBridgeTransferTxn(asset, addrTo)],
      sk
    );
  }
};

main();
