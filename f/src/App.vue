<script>

import * as tus from 'tus-js-client';
import forge from 'node-forge';

export default {
  data() {
    return {
      title: 'abcx',
      transmitted: [],
      progress: 0.0,
      performance: 0.0,
    }
  },
  mounted() {
  },
  methods: {
    digest(file) {
      return new Promise((resolve, reject) => {
        let md = forge.md.sha256.create();
        const CHUNK_SIZE = 4 * 1024 * 1024;

        let fileReader = new FileReader();

        let read = 0;
        fileReader.onload = (e) => {
          if (!e || !e.target || !e.target.result) {
            let err = new Error('invalid event.target.result');
            console.log(err.message);
            return;
          }

          read += e.target.result.length;
          md.update(e.target.result);
          seek();
        };

        let fileSize = file.size;
        let start = 0, end = 0;

        let seek = () => {
          let now = new Date();
          this.performance = (read * 1.0 / (now.getTime() - beginTime.getTime())) * 1000 / (1024 * 1024);

          this.progress = ((read * 1.0) / fileSize * 100).toFixed(2);
          if (read >= fileSize) {
            console.log(`read completed: ${read}, fileSize: ${fileSize}`);
            resolve(md.digest().toHex());
            return;
          }

          end += CHUNK_SIZE;
          end = end < fileSize ? end : fileSize + 1;
          let slice = file.slice(start, end);

          fileReader.readAsBinaryString(slice);
          start = end;
        };

        let beginTime = new Date();
        seek();
      });
    },
    sliceRead(e) {
      if (!e.target.files || !(e.target.files[0])) {
        console.log('no file(s) selected');
        return;
      }

      let beginTime = new Date();
      this.digest(e.target.files[0]).then(v => {
        console.log(`checksum: ${v}`);
        let endTime = new Date();
        let seconds = ((endTime.getTime() - beginTime.getTime()) / 1000.0).toFixed(3);
        console.log(`seconds: ${seconds}`);
      }).catch(err => {
        console.log(err);
      });
    },


    transport(files) {
      return new Promise((resolve, reject) => {
        if (!files || files.length == 0) {
          let errMsg = 'invalid files';
          let err = new Error(errMsg);
          console.log(err.message);
          reject(err);
          return;
        }

        let transmitted = [];
        for (let i = 0; i < files.length; i++) {
          let f = files[i];
          if (!f.name || !f.size) {
            let err = new Error(`invalid files[${i}]`);
            console.log(err.message);
            reject(err);
            return;
          }

          this.digest(f).then(checksum => {
            let metadata = {
              filename: f.name,
              filetype: f.type,
              filesize: f.size,
              lastModified: f.lastModified,
              checksum,
            };

            let upload = new tus.Upload(f, {
              endpoint: 'http://localhost:1080/files/',
              metadata,
              onError: (err) => {
                console.log(err);
                reject(err);
              },
              onAfterResponse: (req, resp) => { },
              onSuccess: () => {
                metadata.url = upload.url;
                transmitted.push(metadata);
                if (transmitted.length !== files.length) {
                  return;
                }

                resolve(transmitted);
              },
              onProgress: (bytesUploaded, bytesTotal) => {
                console.log(`${bytesUploaded}/${bytesTotal}`)
              },
            });

            // Check if there are any previous uploads to continue.
            upload.findPreviousUploads().then((previousUploads) => {
              // Found previous uploads so we select the first one.
              if (previousUploads.length) {
                upload.resumeFromPreviousUpload(previousUploads[0])
              }

              // Start the upload
              upload.start()
            });
          }).catch(err => {
            console.log(err);
            reject(err);
          });
        }
      });
    },

    f(e) {
      if (!e.target.files || !(e.target.files[0])) {
        console.log('no file(s) selected');
        return;
      }

      this.transport(e.target.files).then(v => {
        this.transmitted = v;
        console.log(v);
      }).catch(err => {
        console.log(err);
      });

    }
  }
}
</script>

<template>
  {{ title }}
  <h1 class="status">local: {{ progress }}%, {{ performance.toFixed(2) }}MB</h1>
  <input type="file" @change="sliceRead" multiple />
  <h1>remote:</h1>
  <input type="file" @change="f" multiple />
</template>

<style lang="scss" scoped>
.status {
  font-family: "Courier New", Courier, monospace;
}
</style>