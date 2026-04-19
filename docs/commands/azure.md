# `sku azure`

| Leaf | Shard | Key flags (`price`) |
|---|---|---|
| `vm` | `azure-vm` | `--arm-sku-name`, `--region`, `--os` (`linux`,`windows`) |
| `sql` | `azure-sql` | `--sku-name`, `--region`, `--deployment-option` |
| `blob` | `azure-blob` | `--tier` (`hot`,`cool`,`archive`), `--region` |
| `functions` | `azure-functions` | `--architecture`, `--region` |
| `disks` | `azure-disks` | `--disk-type` (`premium-ssd`,`standard-ssd`,`standard-hdd`), `--region` |

## Examples

```bash
sku azure vm price        --arm-sku-name Standard_D2_v3 --region eastus --os linux --preset agent
sku azure sql price       --sku-name GP_Gen5_2 --region eastus --deployment-option single-az
sku azure blob price      --tier hot --region eastus
sku azure functions price --architecture x86_64 --region eastus
sku azure disks price     --disk-type premium-ssd --region eastus
```

Azure region names use the short form (`eastus`, `westeurope`). The region-normalization layer maps `--regions us-east` prefixes to Azure's `eastus` in `compare`.
