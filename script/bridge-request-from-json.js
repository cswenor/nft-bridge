import algosdk, {
  mnemonicToSecretKey,
  signTransaction,
  makePaymentTxnWithSuggestedParamsFromObject,
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

console.log("node", node);

console.log("address", address);
console.log("address2", address2);
console.log("address3", address3);
console.log("bridgeAddress", bridgeAddress);

const [algodClient, indexerClient] = getAlgorandClients(node);

const makeBridgeRequestTxn = async (asset, addrTo) => {
  const transactionFee = 0.201;
  const serviceFee = 0.099;
  const totalFee = transactionFee + serviceFee;
  const req = {
    assetId: asset.index,
    to: addrTo,
    amount: 1,
  };
  return makePaymentTxnWithSuggestedParamsFromObject({
    from: address,
    to: bridgeAddress,
    amount: Math.floor(totalFee * 1e6),
    note: new Uint8Array(Buffer.from(JSON.stringify(req))),
    suggestedParams: await algodClient.getTransactionParams().do(),
  });
};

const main = async () => {
  console.log("Do the thing!");
  const n = 6;
  for (const asset of data.slice(n, n + 1)) {
    console.log(`Requesting ${asset.params.name} (${asset.index})`);
    const res = await signSendAndConfirm(
      algodClient,
      [await makeBridgeRequestTxn(asset, addrTo)],
      sk
    );
    console.log(res);
  }
};

main();
